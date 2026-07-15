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

func TestHTTP_GetLogPerformance_returnsFilteredModelSeries(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.Channel{}))
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
	now := time.Now().Unix()
	require.NoError(t, db.Create(&model.Channel{Id: 8, Name: "渠道甲"}).Error)
	require.NoError(t, db.Create(&model.Log{
		CreatedAt: now, Type: model.LogTypeConsume, ModelName: "model-a", ChannelId: 8,
		CompletionTokens: 120, UseTime: 2, IsStream: true,
		Other: common.MapToJsonStr(map[string]any{"frt": 500}),
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		CreatedAt: now, Type: model.LogTypeConsume, ModelName: "model-b", ChannelId: 8,
		CompletionTokens: 50, UseTime: 2, IsStream: true,
		Other: common.MapToJsonStr(map[string]any{"frt": 500}),
	}).Error)
	r := gin.New()
	r.GET("/api/log/performance", GetLogPerformance)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf(
		"/api/log/performance?start_timestamp=%d&end_timestamp=%d&model_name=model-a&group_by=model",
		now-60, now+60,
	), nil)
	recorder := httptest.NewRecorder()

	// When
	r.ServeHTTP(recorder, req)

	// Then
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Series []struct {
				Name string `json:"name"`
			} `json:"series"`
			Summary struct {
				Requests int `json:"requests"`
			} `json:"summary"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, 1, payload.Data.Summary.Requests)
	require.Equal(t, "model-a", payload.Data.Series[0].Name)
}
