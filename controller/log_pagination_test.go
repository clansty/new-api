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

func TestGetLogByKeySupportsPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalDB, originalLogDB := model.DB, model.LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = originalDB, originalLogDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	logs := make([]model.Log, 3)
	for i := range logs {
		logs[i] = model.Log{TokenId: 7, Type: model.LogTypeConsume, Quota: i + 1}
	}
	require.NoError(t, db.Create(&logs).Error)

	r := gin.New()
	r.GET("/api/log/token", func(c *gin.Context) {
		c.Set("token_id", 7)
		c.Set("token_root_id", 7)
		GetLogByKey(c)
	})
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/log/token?p=2&page_size=1", nil))

	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Page     int `json:"page"`
			PageSize int `json:"page_size"`
			Total    int `json:"total"`
			Items    []struct {
				Quota int `json:"quota"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, payload.Success)
	require.Equal(t, 2, payload.Data.Page)
	require.Equal(t, 1, payload.Data.PageSize)
	require.Equal(t, 3, payload.Data.Total)
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, 2, payload.Data.Items[0].Quota)
}
