package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetAllLogs_TextFiltersIgnoreCase_whenInputCaseDiffers(t *testing.T) {
	// Given: 一条文本字段包含混合大小写的使用日志。
	truncateTables(t)
	require.NoError(t, LOG_DB.Create(&Log{
		CreatedAt: time.Now().Unix(),
		Type:      LogTypeConsume,
		Username:  "Alice",
		TokenName: "ProdToken",
		ModelName: "GPT-4O",
		Group:     "Premium",
		RequestId: "Req-ABC",
		Other:     "{}",
	}).Error)

	tests := []struct {
		name      string
		modelName string
		username  string
		tokenName string
		group     string
		requestID string
		wantTotal int64
	}{
		{name: "username", username: "alice", wantTotal: 1},
		{name: "token name", tokenName: "prodtoken", wantTotal: 1},
		{name: "model name", modelName: "gpt-4o", wantTotal: 1},
		{name: "group", group: "premium", wantTotal: 1},
		{name: "request ID", requestID: "req-abc", wantTotal: 1},
		{name: "exact match remains exact", username: "ali", wantTotal: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: 使用不同大小写的文本条件查询日志。
			logs, total, err := GetAllLogs(
				LogTypeUnknown,
				0,
				0,
				tt.modelName,
				tt.username,
				tt.tokenName,
				0,
				10,
				0,
				tt.group,
				tt.requestID,
			)

			// Then: 大小写不影响精确匹配，且不引入隐式通配符。
			require.NoError(t, err)
			require.Equal(t, tt.wantTotal, total)
			require.Len(t, logs, int(tt.wantTotal))
		})
	}
}

func TestSumLogStat_TextFiltersIgnoreCase_whenInputCaseDiffers(t *testing.T) {
	// Given: 一条文本字段包含混合大小写的消费日志。
	truncateTables(t)
	require.NoError(t, LOG_DB.Create(&Log{
		CreatedAt: time.Now().Unix(),
		Type:      LogTypeConsume,
		Username:  "Alice",
		TokenName: "ProdToken",
		ModelName: "GPT-4O",
		Group:     "Premium",
		Quota:     100,
		Other:     "{}",
	}).Error)

	// When: 使用全部小写的文本条件查询日志统计。
	stat, err := SumLogStat(LogStatQuery{
		Username:  "alice",
		TokenName: "prodtoken",
		ModelName: "gpt-4o",
		Group:     "premium",
	})

	// Then: 统计与日志列表采用相同的大小写不敏感语义。
	require.NoError(t, err)
	require.Equal(t, 100, stat.Quota)
}
