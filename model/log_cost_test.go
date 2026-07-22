package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecordConsumeLogCost_whenAutomaticUpstreamRateExists(t *testing.T) {
	// Given: 渠道同时有自动倍率和手动兜底倍率。
	truncateTables(t)
	automaticRate := 0.45
	manualRate := 0.6
	channel := Channel{
		Name:                         "cost-auto",
		Key:                          "test-key",
		UpstreamRateMultiplier:       &automaticRate,
		ManualUpstreamRateMultiplier: &manualRate,
	}
	require.NoError(t, DB.Create(&channel).Error)

	// When: 记录原价为 1000 额度的请求。
	RecordConsumeLog(newLogUserAgentTestContext("cost-test"), 1, RecordConsumeLogParams{
		ChannelId:     channel.Id,
		OriginalQuota: 1000,
		Other:         map[string]any{},
	})

	// Then: 自动倍率优先，成本冻结为 450 额度。
	var log Log
	require.NoError(t, LOG_DB.Order("id desc").First(&log).Error)
	require.NotNil(t, log.Cost)
	require.InDelta(t, 450, *log.Cost, 0.0001)
}

func TestRecordConsumeLogCost_whenOnlyManualUpstreamRateExists(t *testing.T) {
	// Given: 渠道仅配置了手动上游倍率。
	truncateTables(t)
	manualRate := 0.6
	channel := Channel{
		Name:                         "cost-manual",
		Key:                          "test-key",
		ManualUpstreamRateMultiplier: &manualRate,
	}
	require.NoError(t, DB.Create(&channel).Error)

	// When: 记录原价为 1000 额度的请求。
	RecordConsumeLog(newLogUserAgentTestContext("cost-test"), 1, RecordConsumeLogParams{
		ChannelId:     channel.Id,
		OriginalQuota: 1000,
		Other:         map[string]any{},
	})

	// Then: 手动倍率用于计算成本。
	var log Log
	require.NoError(t, LOG_DB.Order("id desc").First(&log).Error)
	require.NotNil(t, log.Cost)
	require.InDelta(t, 600, *log.Cost, 0.0001)
}

func TestRecordConsumeLogCost_whenUpstreamRateIsMissing(t *testing.T) {
	// Given: 渠道没有任何上游倍率。
	truncateTables(t)
	channel := Channel{Name: "cost-missing", Key: "test-key"}
	require.NoError(t, DB.Create(&channel).Error)

	// When: 记录请求。
	RecordConsumeLog(newLogUserAgentTestContext("cost-test"), 1, RecordConsumeLogParams{
		ChannelId:     channel.Id,
		OriginalQuota: 1000,
		Other:         map[string]any{},
	})

	// Then: 成本保持未知，不把缺失倍率误记为免费。
	var log Log
	require.NoError(t, LOG_DB.Order("id desc").First(&log).Error)
	require.Nil(t, log.Cost)
}

func TestRecordTaskBillingLogCost_whenOriginalQuotaIsProvided(t *testing.T) {
	// Given: 异步任务渠道配置了上游倍率，结算日志携带未分组原价。
	truncateTables(t)
	rate := 0.5
	channel := Channel{Name: "task-cost", Key: "test-key", UpstreamRateMultiplier: &rate}
	require.NoError(t, DB.Create(&channel).Error)
	originalQuota := 200.0

	// When: 记录任务补扣日志。
	RecordTaskBillingLog(RecordTaskBillingLogParams{
		UserId:        1,
		LogType:       LogTypeConsume,
		ChannelId:     channel.Id,
		OriginalQuota: &originalQuota,
	})

	// Then: 补扣成本使用同一上游倍率冻结。
	var log Log
	require.NoError(t, LOG_DB.Order("id desc").First(&log).Error)
	require.NotNil(t, log.Cost)
	require.InDelta(t, 100, *log.Cost, 0.0001)
}
