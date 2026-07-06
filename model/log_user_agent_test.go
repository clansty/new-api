package model

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newLogUserAgentTestContext(userAgent string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("User-Agent", userAgent)
	c.Request = req
	c.Set("username", "agent-user")
	c.Set(common.RequestIdKey, "request-id")
	return c
}

func TestRecordConsumeLog_RecordsUserAgent_whenHeaderPresent(t *testing.T) {
	// Given: 带 User-Agent 的请求上下文。
	userAgent := "new-api-test-agent/1.0"
	userID := 987654
	c := newLogUserAgentTestContext(userAgent)

	// When: 记录一条消费日志。
	RecordConsumeLog(c, userID, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     10,
		CompletionTokens: 5,
		ModelName:        "test-model",
		TokenName:        "test-token",
		Quota:            1,
		Content:          "test-content",
		TokenId:          2,
		UseTimeSeconds:   1,
		IsStream:         false,
		Group:            "default",
		Other:            map[string]any{},
	})

	// Then: 日志保留原始 User-Agent。
	var log Log
	require.NoError(t, LOG_DB.Where("user_id = ? AND type = ?", userID, LogTypeConsume).First(&log).Error)
	require.Equal(t, userAgent, log.UserAgent)
}

func TestRecordErrorLog_RecordsUserAgent_whenHeaderPresent(t *testing.T) {
	// Given: 带 User-Agent 的请求上下文。
	userAgent := "new-api-error-agent/1.0"
	userID := 987655
	c := newLogUserAgentTestContext(userAgent)

	// When: 记录一条错误日志。
	RecordErrorLog(c, userID, 1, "test-model", "test-token", "test-error", 2, 1, false, "default", map[string]any{})

	// Then: 日志保留原始 User-Agent。
	var log Log
	require.NoError(t, LOG_DB.Where("user_id = ? AND type = ?", userID, LogTypeError).First(&log).Error)
	require.Equal(t, userAgent, log.UserAgent)
}
