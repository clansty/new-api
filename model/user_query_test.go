package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeUser 在测试库中插入一个用户并返回其自增 id。
// username / aff_code 均带唯一索引，故每个用户必须取不同值。
func makeUser(t *testing.T, name string, quota int, usedQuota int, status int, pinned bool, deleted bool) int {
	t.Helper()
	u := &User{
		Username:  name,
		Password:  "placeholder-pass",
		AffCode:   "aff-" + name,
		Quota:     quota,
		UsedQuota: usedQuota,
		Status:    status,
		Group:     "default",
	}
	if pinned {
		u.PinnedTime = time.Now().Unix()
	}
	require.NoError(t, DB.Create(u).Error)
	if deleted {
		require.NoError(t, DB.Delete(u).Error) // 软删除，写入 deleted_at
	}
	return u.Id
}

func idsOf(users []*User) []int {
	ids := make([]int, len(users))
	for i, u := range users {
		ids[i] = u.Id
	}
	return ids
}

// seedUsers 构造一组覆盖各种额度/状态/置顶/删除组合的用户，返回名称到 id 的映射。
func seedUsers(t *testing.T) map[string]int {
	t.Helper()
	ids := map[string]int{}
	ids["full"] = makeUser(t, "full", 100, 0, 1, false, false)   // 满额度：从未消费
	ids["zero"] = makeUser(t, "zero", 0, 50, 1, false, false)    // 0额度：已耗尽
	ids["active"] = makeUser(t, "active", 50, 50, 1, false, false)
	ids["disabled"] = makeUser(t, "disabled", 200, 10, 2, false, false)
	ids["deleted"] = makeUser(t, "deleted", 30, 5, 1, false, true)
	ids["pinned"] = makeUser(t, "pinned", 500, 500, 1, true, false)
	return ids
}

func TestGetUsers_HideZeroQuota(t *testing.T) {
	truncateTables(t)
	ids := seedUsers(t)

	users, total, err := GetUsers(UserQueryParams{HideZeroQuota: true}, 0, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 5, total)
	for _, u := range users {
		assert.NotEqual(t, ids["zero"], u.Id, "quota==0 用户应被隐藏")
		assert.NotZero(t, u.Quota)
	}
}

func TestGetUsers_HideFullQuota(t *testing.T) {
	truncateTables(t)
	ids := seedUsers(t)

	users, total, err := GetUsers(UserQueryParams{HideFullQuota: true}, 0, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 5, total)
	for _, u := range users {
		assert.NotEqual(t, ids["full"], u.Id, "used_quota==0（满额度）用户应被隐藏")
		assert.NotZero(t, u.UsedQuota)
	}
}

func TestGetUsers_HideDeleted(t *testing.T) {
	truncateTables(t)
	ids := seedUsers(t)

	users, total, err := GetUsers(UserQueryParams{HideDeleted: true}, 0, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 5, total)
	for _, u := range users {
		assert.NotEqual(t, ids["deleted"], u.Id, "已注销用户应被隐藏")
	}
}

func TestGetUsers_AllFiltersCombined(t *testing.T) {
	truncateTables(t)
	ids := seedUsers(t)

	users, total, err := GetUsers(UserQueryParams{
		HideZeroQuota: true,
		HideFullQuota: true,
		HideDeleted:   true,
	}, 0, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	got := idsOf(users)
	assert.ElementsMatch(t, []int{ids["active"], ids["disabled"], ids["pinned"]}, got)
}

func TestGetUsers_DefaultSort_PinnedFirstThenIdDesc(t *testing.T) {
	truncateTables(t)
	ids := seedUsers(t)

	users, _, err := GetUsers(UserQueryParams{}, 0, 100)
	require.NoError(t, err)
	want := []int{ids["pinned"], ids["deleted"], ids["disabled"], ids["active"], ids["zero"], ids["full"]}
	assert.Equal(t, want, idsOf(users))
}

func TestGetUsers_DefaultSort_Reverse(t *testing.T) {
	truncateTables(t)
	ids := seedUsers(t)

	users, _, err := GetUsers(UserQueryParams{SortOrder: "asc"}, 0, 100)
	require.NoError(t, err)
	// 置顶仍在最前，其余按 id 升序
	want := []int{ids["pinned"], ids["full"], ids["zero"], ids["active"], ids["disabled"], ids["deleted"]}
	assert.Equal(t, want, idsOf(users))
}

func TestGetUsers_SortRemainingQuota(t *testing.T) {
	truncateTables(t)
	ids := seedUsers(t)

	desc, _, err := GetUsers(UserQueryParams{SortBy: UserSortRemainingQuota}, 0, 100)
	require.NoError(t, err)
	want := []int{ids["pinned"], ids["disabled"], ids["full"], ids["active"], ids["deleted"], ids["zero"]}
	assert.Equal(t, want, idsOf(desc))

	asc, _, err := GetUsers(UserQueryParams{SortBy: UserSortRemainingQuota, SortOrder: "asc"}, 0, 100)
	require.NoError(t, err)
	wantAsc := []int{ids["pinned"], ids["zero"], ids["deleted"], ids["active"], ids["full"], ids["disabled"]}
	assert.Equal(t, wantAsc, idsOf(asc))
}

func TestGetUsers_SortMaxQuota(t *testing.T) {
	truncateTables(t)
	ids := seedUsers(t)

	// max = quota + used_quota: full=100, zero=50, active=100, disabled=210, deleted=35, pinned=1000
	desc, _, err := GetUsers(UserQueryParams{SortBy: UserSortMaxQuota}, 0, 100)
	require.NoError(t, err)
	// 100 平局（active id > full id）按 id desc 取 active 在前
	want := []int{ids["pinned"], ids["disabled"], ids["active"], ids["full"], ids["zero"], ids["deleted"]}
	assert.Equal(t, want, idsOf(desc))
}

func TestGetUsers_PaginationWithDefaultSort(t *testing.T) {
	truncateTables(t)
	ids := seedUsers(t)

	page1, total, err := GetUsers(UserQueryParams{}, 0, 2)
	require.NoError(t, err)
	assert.EqualValues(t, 6, total)
	assert.Equal(t, []int{ids["pinned"], ids["deleted"]}, idsOf(page1))

	page2, _, err := GetUsers(UserQueryParams{}, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, []int{ids["disabled"], ids["active"]}, idsOf(page2))

	page3, _, err := GetUsers(UserQueryParams{}, 4, 2)
	require.NoError(t, err)
	assert.Equal(t, []int{ids["zero"], ids["full"]}, idsOf(page3))
}

func TestGetUsers_InflightSortReturnsAllRows(t *testing.T) {
	truncateTables(t)
	seedUsers(t)

	// inflight 排序无法在 SQL 完成，GetUsers 必须忽略分页返回全部，交由控制层处理
	users, total, err := GetUsers(UserQueryParams{SortBy: UserSortInflight}, 0, 2)
	require.NoError(t, err)
	assert.EqualValues(t, 6, total)
	assert.Len(t, users, 6, "inflight 排序应返回全部过滤结果而非分页子集")
}

func TestGetUsers_KeywordWithFilter(t *testing.T) {
	truncateTables(t)
	seedUsers(t)
	// 额外插入一个匹配关键字但已耗尽的用户，确认过滤与搜索同时生效
	makeUser(t, "vipzero", 0, 80, 1, false, false)

	users, total, err := GetUsers(UserQueryParams{Keyword: "vip", HideZeroQuota: true}, 0, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 0, total, "关键字命中但额度为0时应被过滤")
	assert.Empty(t, users)

	matched, total, err := GetUsers(UserQueryParams{Keyword: "vip"}, 0, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Len(t, matched, 1)
}
