package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHTTP_LogVisibility_hidesSuccessfulRetryErrorsFromUser(t *testing.T) {
	// Given: 一次自动重试成功的请求包含失败尝试和最终消费日志。
	gin.SetMode(gin.TestMode)
	originalDB, originalLogDB := model.DB, model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	common.UsingSQLite = true
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = originalDB, originalLogDB
		common.UsingSQLite = originalUsingSQLite
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
	const (
		userID    = 301
		requestID = "http-retry-success"
	)
	require.NoError(t, db.Create(&[]model.Log{
		{UserId: userID, Type: model.LogTypeError, RequestId: requestID},
		{UserId: userID, Type: model.LogTypeConsume, RequestId: requestID},
	}).Error)
	require.NoError(t, db.Model(&model.Log{}).
		Where("user_id = ? AND request_id = ? AND type = ?", userID, requestID, model.LogTypeError).
		Update("user_visible", false).Error)

	r := gin.New()
	r.GET("/user", func(c *gin.Context) {
		c.Set("id", userID)
		GetUserLogs(c)
	})
	r.GET("/admin", GetAllLogs)

	// When: 用户和管理员分别访问日志接口。
	userRecorder := httptest.NewRecorder()
	r.ServeHTTP(userRecorder, httptest.NewRequest(http.MethodGet, "/user?p=1&page_size=10", nil))
	adminRecorder := httptest.NewRecorder()
	r.ServeHTTP(adminRecorder, httptest.NewRequest(http.MethodGet, "/admin?p=1&page_size=10", nil))

	// Then: 用户只看到成功日志，管理员看到失败和成功两条记录。
	require.Equal(t, http.StatusOK, userRecorder.Code)
	require.Equal(t, http.StatusOK, adminRecorder.Code)
	var userPayload, adminPayload struct {
		Success bool `json:"success"`
		Data    struct {
			Total int `json:"total"`
			Items []struct {
				Type int `json:"type"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(userRecorder.Body.Bytes(), &userPayload))
	require.NoError(t, common.Unmarshal(adminRecorder.Body.Bytes(), &adminPayload))
	require.True(t, userPayload.Success)
	require.Equal(t, 1, userPayload.Data.Total)
	require.Len(t, userPayload.Data.Items, 1)
	require.Equal(t, model.LogTypeConsume, userPayload.Data.Items[0].Type)
	require.True(t, adminPayload.Success)
	require.Equal(t, 2, adminPayload.Data.Total)
	require.Len(t, adminPayload.Data.Items, 2)
}

func TestHTTP_LogVisibility_keepsFinalFailureVisibleToUser(t *testing.T) {
	// Given: 请求最终失败，错误日志没有成功重试标记。
	gin.SetMode(gin.TestMode)
	originalDB, originalLogDB := model.DB, model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	common.UsingSQLite = true
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = originalDB, originalLogDB
		common.UsingSQLite = originalUsingSQLite
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
	const userID = 302
	require.NoError(t, db.Create(&model.Log{UserId: userID, Type: model.LogTypeError, RequestId: "http-final-failure"}).Error)
	r := gin.New()
	r.GET("/user", func(c *gin.Context) {
		c.Set("id", userID)
		GetUserLogs(c)
	})

	// When: 用户访问最终失败请求的日志。
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/user?p=1&page_size=10", nil))

	// Then: 最终失败仍然展示给用户。
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, 1, payload.Data.Total)
}
