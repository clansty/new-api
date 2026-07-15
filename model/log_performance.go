package model

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type PerformanceGroup string

const (
	PerformanceGroupModel   PerformanceGroup = "model"
	PerformanceGroupChannel PerformanceGroup = "channel"
)

type LogPerformanceQuery struct {
	StartTimestamp int64
	EndTimestamp   int64
	ModelName      string
	ChannelID      int
	GroupBy        PerformanceGroup
}

type PerformanceMetricStats struct {
	Samples int     `json:"samples"`
	P50     float64 `json:"p50"`
	P95     float64 `json:"p95"`
	P99     float64 `json:"p99"`
}

type PerformancePoint struct {
	Timestamp int64                  `json:"timestamp"`
	TTFT      PerformanceMetricStats `json:"ttft"`
	TPS       PerformanceMetricStats `json:"tps"`
}

type PerformanceSeries struct {
	Key       string             `json:"key"`
	Name      string             `json:"name"`
	ModelName string             `json:"model_name,omitempty"`
	ChannelID int                `json:"channel_id,omitempty"`
	Points    []PerformancePoint `json:"points"`
}

type PerformanceChannel struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type LogPerformanceResult struct {
	BucketSeconds int64                `json:"bucket_seconds"`
	Series        []PerformanceSeries  `json:"series"`
	Models        []string             `json:"models"`
	Channels      []PerformanceChannel `json:"channels"`
	Summary       struct {
		Requests int                    `json:"requests"`
		TTFT     PerformanceMetricStats `json:"ttft"`
		TPS      PerformanceMetricStats `json:"tps"`
	} `json:"summary"`
}

type logPerformanceRow struct {
	CreatedAt        int64  `gorm:"column:created_at"`
	ModelName        string `gorm:"column:model_name"`
	ChannelID        int    `gorm:"column:channel_id"`
	CompletionTokens int    `gorm:"column:completion_tokens"`
	UseTime          int    `gorm:"column:use_time"`
	IsStream         bool   `gorm:"column:is_stream"`
	Other            string `gorm:"column:other"`
}

type performanceValues struct {
	ttft []float64
	tps  []float64
}

func GetLogPerformance(query LogPerformanceQuery) (LogPerformanceResult, error) {
	var models []string
	if err := LOG_DB.Model(&Log{}).
		Where("type = ? AND created_at >= ? AND created_at <= ? AND model_name <> ?", LogTypeConsume, query.StartTimestamp, query.EndTimestamp, "").
		Distinct().Order("model_name").Pluck("model_name", &models).Error; err != nil {
		return LogPerformanceResult{}, fmt.Errorf("查询性能模型维度失败: %w", err)
	}
	var channelIDs []int
	if err := LOG_DB.Model(&Log{}).
		Where("type = ? AND created_at >= ? AND created_at <= ? AND channel_id <> ?", LogTypeConsume, query.StartTimestamp, query.EndTimestamp, 0).
		Distinct().Order("channel_id").Pluck("channel_id", &channelIDs).Error; err != nil {
		return LogPerformanceResult{}, fmt.Errorf("查询性能渠道维度失败: %w", err)
	}

	tx := LOG_DB.Model(&Log{}).
		Select("created_at, model_name, channel_id, completion_tokens, use_time, is_stream, other").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, query.StartTimestamp, query.EndTimestamp)
	if query.ModelName != "" {
		tx = tx.Where("model_name = ?", query.ModelName)
	}
	if query.ChannelID != 0 {
		tx = tx.Where("channel_id = ?", query.ChannelID)
	}
	var rows []logPerformanceRow
	if err := tx.Order("created_at").Find(&rows).Error; err != nil {
		return LogPerformanceResult{}, fmt.Errorf("查询性能日志失败: %w", err)
	}

	channelNames, err := getPerformanceChannelNames(channelIDs)
	if err != nil {
		return LogPerformanceResult{}, err
	}
	result := aggregateLogPerformance(rows, query.GroupBy, performanceBucketSeconds(query.EndTimestamp-query.StartTimestamp), channelNames)
	result.Models = models
	for _, id := range channelIDs {
		if id == 0 {
			continue
		}
		name := channelNames[id]
		if name == "" {
			name = fmt.Sprintf("#%d", id)
		}
		result.Channels = append(result.Channels, PerformanceChannel{ID: id, Name: name})
	}
	return result, nil
}

func getPerformanceChannelNames(channelIDs []int) (map[int]string, error) {
	names := make(map[int]string, len(channelIDs))
	if len(channelIDs) == 0 {
		return names, nil
	}
	var channels []struct {
		ID   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if err := DB.Table("channels").Select("id, name").Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("查询性能渠道名称失败: %w", err)
	}
	for _, channel := range channels {
		names[channel.ID] = channel.Name
	}
	return names, nil
}

func performanceBucketSeconds(duration int64) int64 {
	switch {
	case duration <= 2*int64(24*time.Hour/time.Second):
		return int64(time.Hour / time.Second)
	case duration <= 7*int64(24*time.Hour/time.Second):
		return int64(6 * time.Hour / time.Second)
	default:
		return int64(24 * time.Hour / time.Second)
	}
}

func aggregateLogPerformance(rows []logPerformanceRow, groupBy PerformanceGroup, bucketSeconds int64, channelNames map[int]string) LogPerformanceResult {
	result := LogPerformanceResult{BucketSeconds: bucketSeconds}
	buckets := make(map[string]map[int64]*performanceValues)
	seriesMeta := make(map[string]PerformanceSeries)
	allValues := performanceValues{}

	for _, row := range rows {
		key, name := performanceSeriesIdentity(row, groupBy, channelNames)
		if _, ok := buckets[key]; !ok {
			buckets[key] = make(map[int64]*performanceValues)
			seriesMeta[key] = PerformanceSeries{Key: key, Name: name, ModelName: row.ModelName, ChannelID: row.ChannelID}
		}
		timestamp := row.CreatedAt / bucketSeconds * bucketSeconds
		if buckets[key][timestamp] == nil {
			buckets[key][timestamp] = &performanceValues{}
		}
		values := buckets[key][timestamp]
		var timing struct {
			FRT *float64 `json:"frt"`
		}
		if err := common.UnmarshalJsonStr(row.Other, &timing); err == nil && timing.FRT != nil && *timing.FRT >= 0 {
			values.ttft = append(values.ttft, *timing.FRT)
			allValues.ttft = append(allValues.ttft, *timing.FRT)
			outputSeconds := float64(row.UseTime) - *timing.FRT/1000
			if row.IsStream && row.CompletionTokens > 0 && outputSeconds > 0 {
				tps := float64(row.CompletionTokens) / outputSeconds
				values.tps = append(values.tps, tps)
				allValues.tps = append(allValues.tps, tps)
			}
		}
	}

	keys := make([]string, 0, len(seriesMeta))
	for key := range seriesMeta {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return seriesMeta[keys[i]].Name < seriesMeta[keys[j]].Name })
	for _, key := range keys {
		series := seriesMeta[key]
		timestamps := make([]int64, 0, len(buckets[key]))
		for timestamp := range buckets[key] {
			timestamps = append(timestamps, timestamp)
		}
		sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
		for _, timestamp := range timestamps {
			values := buckets[key][timestamp]
			series.Points = append(series.Points, PerformancePoint{Timestamp: timestamp, TTFT: metricStats(values.ttft), TPS: metricStats(values.tps)})
		}
		if groupBy == PerformanceGroupModel {
			series.ChannelID = 0
		} else {
			series.ModelName = ""
		}
		result.Series = append(result.Series, series)
	}
	result.Summary.Requests = len(rows)
	result.Summary.TTFT = metricStats(allValues.ttft)
	result.Summary.TPS = metricStats(allValues.tps)
	return result
}

func performanceSeriesIdentity(row logPerformanceRow, groupBy PerformanceGroup, channelNames map[int]string) (string, string) {
	if groupBy == PerformanceGroupChannel {
		name := channelNames[row.ChannelID]
		if name == "" {
			name = "未知渠道"
		}
		return fmt.Sprintf("channel:%d", row.ChannelID), fmt.Sprintf("%s (#%d)", name, row.ChannelID)
	}
	return "model:" + row.ModelName, row.ModelName
}

func metricStats(values []float64) PerformanceMetricStats {
	if len(values) == 0 {
		return PerformanceMetricStats{}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return PerformanceMetricStats{
		Samples: len(sorted),
		P50:     nearestRank(sorted, 0.50),
		P95:     nearestRank(sorted, 0.95),
		P99:     nearestRank(sorted, 0.99),
	}
}

func nearestRank(sorted []float64, percentile float64) float64 {
	index := int(math.Ceil(float64(len(sorted))*percentile)) - 1
	if index < 0 {
		index = 0
	}
	return sorted[index]
}
