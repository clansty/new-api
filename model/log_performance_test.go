package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestAggregateLogPerformance_whenRowsContainStreamTiming(t *testing.T) {
	// Given
	rows := []logPerformanceRow{
		{CreatedAt: 3700, ModelName: "model-a", ChannelID: 1, CompletionTokens: 90, UseTime: 2, IsStream: true, Other: common.MapToJsonStr(map[string]any{"frt": 500})},
		{CreatedAt: 3750, ModelName: "model-a", ChannelID: 1, CompletionTokens: 180, UseTime: 3, IsStream: true, Other: common.MapToJsonStr(map[string]any{"frt": 1000})},
		{CreatedAt: 3800, ModelName: "model-b", ChannelID: 2, CompletionTokens: 100, UseTime: 1, IsStream: false, Other: common.MapToJsonStr(map[string]any{"frt": 250})},
		{CreatedAt: 3850, ModelName: "model-b", ChannelID: 2, CompletionTokens: 100, UseTime: 1, IsStream: true, Other: "{}"},
	}

	// When
	result := aggregateLogPerformance(rows, PerformanceGroupModel, 3600, map[int]string{1: "channel-a", 2: "channel-b"})

	// Then
	require.Len(t, result.Series, 2)
	require.Equal(t, int64(3600), result.Series[0].Points[0].Timestamp)
	require.InDelta(t, 500, result.Summary.TTFT.P50, 0.001)
	require.InDelta(t, 90, result.Summary.TPS.P95, 0.001)
	require.Equal(t, 4, result.Summary.Requests)
	require.Equal(t, 3, result.Summary.TTFT.Samples)
	require.Equal(t, 2, result.Summary.TPS.Samples)
}

func TestAggregateLogPerformance_whenGroupingByChannel(t *testing.T) {
	// Given
	rows := []logPerformanceRow{
		{CreatedAt: 100, ModelName: "model-a", ChannelID: 7, CompletionTokens: 50, UseTime: 2, IsStream: true, Other: common.MapToJsonStr(map[string]any{"frt": 1000})},
		{CreatedAt: 110, ModelName: "model-b", ChannelID: 7, CompletionTokens: 50, UseTime: 2, IsStream: true, Other: common.MapToJsonStr(map[string]any{"frt": 1000})},
	}

	// When
	result := aggregateLogPerformance(rows, PerformanceGroupChannel, 60, map[int]string{7: "主渠道"})

	// Then
	require.Len(t, result.Series, 1)
	require.Equal(t, "主渠道 (#7)", result.Series[0].Name)
	require.Equal(t, 2, result.Series[0].Points[0].TTFT.Samples)
}
