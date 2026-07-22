package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetCostStatsAggregatesNetCostAndExcludesUnknownCost(t *testing.T) {
	// Given: 两个渠道存在消费、退款以及无法统计成本的日志。
	truncateTables(t)
	channels := []Channel{{Name: "渠道一", Key: "key-1"}, {Name: "渠道二", Key: "key-2"}}
	require.NoError(t, DB.Create(&channels).Error)
	cost100 := 100.0
	cost40 := 40.0
	cost25 := 25.0
	require.NoError(t, LOG_DB.Create(&[]Log{
		{Username: "alice", CreatedAt: 7201, Type: LogTypeConsume, ModelName: "model-a", ChannelId: channels[0].Id, Cost: &cost100},
		{Username: "alice", CreatedAt: 7202, Type: LogTypeRefund, ModelName: "model-a", ChannelId: channels[0].Id, Cost: &cost40},
		{Username: "alice", CreatedAt: 7203, Type: LogTypeConsume, ModelName: "model-b", ChannelId: channels[1].Id, Cost: &cost25},
		{Username: "alice", CreatedAt: 7204, Type: LogTypeConsume, ModelName: "model-a", ChannelId: channels[0].Id},
		{Username: "bob", CreatedAt: 7205, Type: LogTypeConsume, ModelName: "model-a", ChannelId: channels[0].Id, Cost: &cost100},
	}).Error)

	// When: 管理员按用户和时间查询成本分布。
	stats, err := GetCostStats(7200, 10800, "alice")

	// Then: 退款抵扣消费、未知成本被排除，且渠道名称来自主库。
	require.NoError(t, err)
	require.Len(t, stats, 2)
	require.Equal(t, "model-a", stats[0].ModelName)
	require.Equal(t, "渠道一", stats[0].ChannelName)
	require.InDelta(t, 60, stats[0].Cost, 0.0001)
	require.Equal(t, int64(7200), stats[0].CreatedAt)
	require.Equal(t, "model-b", stats[1].ModelName)
	require.Equal(t, "渠道二", stats[1].ChannelName)
	require.InDelta(t, 25, stats[1].Cost, 0.0001)
}
