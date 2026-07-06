package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHTTP_GetLogsStat_returnsCacheHitRate_whenRowsReachBatchSize(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	now := time.Now().Unix()
	logs := make([]model.Log, 1000)
	for i := range logs {
		logs[i] = model.Log{
			UserId:       1,
			Username:     "alice",
			CreatedAt:    now,
			Type:         model.LogTypeConsume,
			TokenName:    "token-a",
			ModelName:    "gpt-http-batch-test",
			PromptTokens: 10,
			Other: common.MapToJsonStr(map[string]any{
				"cache_tokens": 5,
			}),
		}
	}
	require.NoError(t, db.CreateInBatches(logs, 100).Error)

	r := gin.New()
	r.GET("/api/log/stat", GetLogsStat)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf(
		"/api/log/stat?type=0&username=alice&token_name=token-a&model_name=gpt-http-batch-test&start_timestamp=%d&end_timestamp=%d&channel=&group=",
		now-1,
		now+1,
	), nil)
	recorder := httptest.NewRecorder()

	// When
	r.ServeHTTP(recorder, req)

	// Then
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			CacheHitRate float64 `json:"cache_hit_rate"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success, payload.Message)
	require.InDelta(t, 5.0/15.0*100, payload.Data.CacheHitRate, 0.0001)
}
