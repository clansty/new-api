package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type userListResp struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total int `json:"total"`
		Items []struct {
			Id        int `json:"id"`
			Quota     int `json:"quota"`
			UsedQuota int `json:"used_quota"`
		} `json:"items"`
	} `json:"data"`
}

func setupUserHTTPTest(t *testing.T) *gin.Engine {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	require.NoError(t, db.AutoMigrate(&model.User{}))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/user/", GetAllUsers)
	r.GET("/api/user/search", SearchUsers)
	return r
}

func seedHTTPUsers(t *testing.T) {
	t.Helper()
	seed := []struct {
		name      string
		quota     int
		usedQuota int
	}{
		{"full", 100, 0},   // 满额度
		{"zero", 0, 50},    // 0额度
		{"active", 50, 50}, // 部分使用
	}
	for _, s := range seed {
		require.NoError(t, model.DB.Create(&model.User{
			Username:  s.name,
			Password:  "placeholder",
			AffCode:   "aff-" + s.name,
			Quota:     s.quota,
			UsedQuota: s.usedQuota,
			Status:    1,
			Group:     "default",
		}).Error)
	}
}

func doUserListRequest(t *testing.T, r *gin.Engine, url string) userListResp {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var out userListResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.True(t, out.Success, out.Message)
	return out
}

func TestHTTP_GetAllUsers_NoFilter(t *testing.T) {
	r := setupUserHTTPTest(t)
	seedHTTPUsers(t)

	resp := doUserListRequest(t, r, "/api/user/?p=1&page_size=100")
	assert.Equal(t, 3, resp.Data.Total)
}

func TestHTTP_GetAllUsers_HideZeroQuota(t *testing.T) {
	r := setupUserHTTPTest(t)
	seedHTTPUsers(t)

	resp := doUserListRequest(t, r, "/api/user/?p=1&page_size=100&hide_zero_quota=true")
	assert.Equal(t, 2, resp.Data.Total)
	for _, item := range resp.Data.Items {
		assert.NotZero(t, item.Quota)
	}
}

func TestHTTP_GetAllUsers_HideFullQuota(t *testing.T) {
	r := setupUserHTTPTest(t)
	seedHTTPUsers(t)

	resp := doUserListRequest(t, r, "/api/user/?p=1&page_size=100&hide_full_quota=true")
	assert.Equal(t, 2, resp.Data.Total)
	for _, item := range resp.Data.Items {
		assert.NotZero(t, item.UsedQuota)
	}
}

func TestHTTP_GetAllUsers_SortRemainingQuotaAsc(t *testing.T) {
	r := setupUserHTTPTest(t)
	seedHTTPUsers(t)

	resp := doUserListRequest(t, r, "/api/user/?p=1&page_size=100&sort_by=remaining_quota&sort_order=asc")
	require.Len(t, resp.Data.Items, 3)
	// 剩余额度升序：0, 50, 100
	assert.Equal(t, 0, resp.Data.Items[0].Quota)
	assert.Equal(t, 50, resp.Data.Items[1].Quota)
	assert.Equal(t, 100, resp.Data.Items[2].Quota)
}

func TestHTTP_GetAllUsers_SortInflightReturnsPage(t *testing.T) {
	r := setupUserHTTPTest(t)
	seedHTTPUsers(t)

	// inflight 走内存排序+手动分页路径，page_size 仍应生效
	resp := doUserListRequest(t, r, "/api/user/?p=1&page_size=2&sort_by=inflight")
	assert.Equal(t, 3, resp.Data.Total)
	assert.Len(t, resp.Data.Items, 2, "内存分页应只返回当前页大小")
}

func TestHTTP_SearchUsers_KeywordWithSort(t *testing.T) {
	r := setupUserHTTPTest(t)
	seedHTTPUsers(t)

	resp := doUserListRequest(t, r, "/api/user/search?keyword=active&p=1&page_size=100&sort_order=desc")
	assert.Equal(t, 1, resp.Data.Total)
	require.Len(t, resp.Data.Items, 1)
	assert.Equal(t, 50, resp.Data.Items[0].Quota)
}
