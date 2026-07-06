package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSumLogStatCacheHitRate_whenCacheReadAndWriteTokensExist(t *testing.T) {
	truncateTables(t)
	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           1,
		Username:         "alice",
		CreatedAt:        now,
		Type:             LogTypeConsume,
		TokenName:        "token-a",
		ModelName:        "gpt-test",
		Quota:            100,
		PromptTokens:     100,
		CompletionTokens: 20,
		ChannelId:        3,
		Group:            "default",
		Other: common.MapToJsonStr(map[string]any{
			"cache_tokens":          50,
			"cache_creation_tokens": 25,
		}),
	}).Error)

	stat, err := SumLogStat(LogStatQuery{
		StartTimestamp:      now - 1,
		EndTimestamp:        now + 1,
		ModelName:           "gpt-test",
		Username:            "alice",
		TokenName:           "token-a",
		Channel:             3,
		Group:               "default",
		IncludeCacheHitRate: true,
	})

	require.NoError(t, err)
	require.Equal(t, 100, stat.Quota)
	require.Equal(t, 1, stat.Rpm)
	require.Equal(t, 120, stat.Tpm)
	require.InDelta(t, 50.0/175.0*100, stat.CacheHitRate, 0.0001)
}

func TestSumLogStatCacheHitRate_whenSplitCacheWriteTokensExist(t *testing.T) {
	truncateTables(t)
	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:       1,
		Username:     "alice",
		CreatedAt:    now,
		Type:         LogTypeConsume,
		TokenName:    "token-a",
		ModelName:    "claude-test",
		PromptTokens: 100,
		ChannelId:    3,
		Group:        "default",
		Other: common.MapToJsonStr(map[string]any{
			"cache_tokens":             50,
			"cache_creation_tokens":    999,
			"cache_creation_tokens_5m": 20,
			"cache_creation_tokens_1h": 30,
		}),
	}).Error)

	stat, err := SumLogStat(LogStatQuery{
		StartTimestamp:      now - 1,
		EndTimestamp:        now + 1,
		ModelName:           "claude-test",
		Username:            "alice",
		TokenName:           "token-a",
		Channel:             3,
		Group:               "default",
		IncludeCacheHitRate: true,
	})

	require.NoError(t, err)
	require.InDelta(t, 50.0/200.0*100, stat.CacheHitRate, 0.0001)
}

func TestSumLogStatCacheHitRate_whenRowsReachBatchSize(t *testing.T) {
	truncateTables(t)
	now := time.Now().Unix()
	logs := make([]Log, logStatBatchSize)
	for i := range logs {
		logs[i] = Log{
			UserId:       1,
			Username:     "alice",
			CreatedAt:    now,
			Type:         LogTypeConsume,
			TokenName:    "token-a",
			ModelName:    "gpt-batch-test",
			PromptTokens: 10,
			Group:        "default",
			Other: common.MapToJsonStr(map[string]any{
				"cache_tokens": 5,
			}),
		}
	}
	require.NoError(t, LOG_DB.CreateInBatches(logs, 100).Error)

	stat, err := SumLogStat(LogStatQuery{
		StartTimestamp:      now - 1,
		EndTimestamp:        now + 1,
		ModelName:           "gpt-batch-test",
		Username:            "alice",
		TokenName:           "token-a",
		Group:               "default",
		IncludeCacheHitRate: true,
	})

	require.NoError(t, err)
	require.InDelta(t, 5.0/15.0*100, stat.CacheHitRate, 0.0001)
}
