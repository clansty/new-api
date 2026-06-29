package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func inflightTestIds(users []*model.User) []int {
	ids := make([]int, len(users))
	for i, u := range users {
		ids[i] = u.Id
	}
	return ids
}

func TestSortUsersByInflight(t *testing.T) {
	newSet := func() []*model.User {
		return []*model.User{
			{Id: 1, InflightCount: 2},
			{Id: 2, InflightCount: 5},
			{Id: 3, InflightCount: 0, PinnedTime: 100}, // 置顶用户，应始终在最前
			{Id: 4, InflightCount: 5},
		}
	}

	desc := newSet()
	sortUsersByInflight(desc, "desc")
	// 置顶(3) 在前；其余按并发降序，5 平局时按 id 降序 → 4,2；最后并发 2 的 1
	assert.Equal(t, []int{3, 4, 2, 1}, inflightTestIds(desc))

	asc := newSet()
	sortUsersByInflight(asc, "asc")
	// 置顶(3) 在前；其余按并发升序 → 并发2 的 1，再到 5 平局按 id 降序 4,2
	assert.Equal(t, []int{3, 1, 4, 2}, inflightTestIds(asc))
}

func TestPaginateUsers(t *testing.T) {
	users := []*model.User{{Id: 1}, {Id: 2}, {Id: 3}, {Id: 4}, {Id: 5}}

	assert.Equal(t, []int{1, 2}, inflightTestIds(paginateUsers(users, 0, 2)))
	assert.Equal(t, []int{3, 4}, inflightTestIds(paginateUsers(users, 2, 2)))
	assert.Equal(t, []int{5}, inflightTestIds(paginateUsers(users, 4, 2)))
	assert.Empty(t, paginateUsers(users, 10, 2))
	assert.Equal(t, []int{1, 2, 3, 4, 5}, inflightTestIds(paginateUsers(users, -1, 100)))
}
