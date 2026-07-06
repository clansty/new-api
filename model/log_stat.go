package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const logStatBatchSize = 1000

type LogStatQuery struct {
	StartTimestamp      int64
	EndTimestamp        int64
	ModelName           string
	Username            string
	TokenName           string
	Channel             int
	Group               string
	IncludeCacheHitRate bool
}

type Stat struct {
	Quota        int     `json:"quota"`
	Rpm          int     `json:"rpm"`
	Tpm          int     `json:"tpm"`
	CacheHitRate float64 `json:"cache_hit_rate,omitempty"`
}

type logInputTokenRow struct {
	ID           int    `gorm:"column:id;primaryKey"`
	PromptTokens int    `gorm:"column:prompt_tokens"`
	Other        string `gorm:"column:other"`
}

type inputTokenStat struct {
	cacheReadTokens  int
	totalInputTokens int
}

func SumLogStat(query LogStatQuery) (stat Stat, err error) {
	quotaQuery, err := buildLogStatQuery(query)
	if err != nil {
		return stat, err
	}
	rpmTpmQuery, err := buildLogStatQuery(query)
	if err != nil {
		return stat, err
	}

	quotaQuery = quotaQuery.Select("COALESCE(SUM(quota), 0) quota")
	rpmTpmQuery = rpmTpmQuery.Select("count(*) rpm, COALESCE(SUM(prompt_tokens), 0) + COALESCE(SUM(completion_tokens), 0) tpm")
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	if err := quotaQuery.Scan(&stat).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := rpmTpmQuery.Scan(&stat).Error; err != nil {
		common.SysError("failed to query rpm/tpm stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if query.IncludeCacheHitRate {
		cacheStat, err := sumInputTokenStat(query)
		if err != nil {
			common.SysError("failed to query log cache stat: " + err.Error())
			return stat, errors.New("查询统计数据失败")
		}
		if cacheStat.totalInputTokens > 0 {
			stat.CacheHitRate = float64(cacheStat.cacheReadTokens) / float64(cacheStat.totalInputTokens) * 100
		}
	}

	return stat, nil
}

func buildLogStatQuery(query LogStatQuery) (*gorm.DB, error) {
	tx := LOG_DB.Table("logs")
	var err error
	tx, err = applyLogTextFilter(tx, "username", query.Username)
	if err != nil {
		return nil, err
	}
	tx, err = applyLogTextFilter(tx, "token_name", query.TokenName)
	if err != nil {
		return nil, err
	}
	tx, err = applyLogTextFilter(tx, "model_name", query.ModelName)
	if err != nil {
		return nil, err
	}
	if query.StartTimestamp != 0 {
		tx = tx.Where("created_at >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp != 0 {
		tx = tx.Where("created_at <= ?", query.EndTimestamp)
	}
	if query.Channel != 0 {
		tx = tx.Where("channel_id = ?", query.Channel)
	}
	if query.Group != "" {
		tx = tx.Where(logGroupCol+" = ?", query.Group)
	}
	return tx.Where("type = ?", LogTypeConsume), nil
}

func sumInputTokenStat(query LogStatQuery) (inputTokenStat, error) {
	tx, err := buildLogStatQuery(query)
	if err != nil {
		return inputTokenStat{}, err
	}

	var rows []logInputTokenRow
	stat := inputTokenStat{}
	err = tx.Select("id", "prompt_tokens", "other").FindInBatches(&rows, logStatBatchSize, func(tx *gorm.DB, batch int) error {
		for _, row := range rows {
			stat.add(row)
		}
		return nil
	}).Error
	return stat, err
}

func (stat *inputTokenStat) add(row logInputTokenRow) {
	other, err := common.StrToMap(row.Other)
	if err != nil {
		stat.totalInputTokens += positiveTokenValue(row.PromptTokens)
		return
	}

	cacheReadTokens := positiveTokenValue(other["cache_tokens"])
	cacheWriteTokens := cacheWriteTokenValue(other)
	stat.cacheReadTokens += cacheReadTokens
	stat.totalInputTokens += positiveTokenValue(row.PromptTokens) + cacheReadTokens + cacheWriteTokens
}

func cacheWriteTokenValue(other map[string]any) int {
	cacheWriteTokens := positiveTokenValue(other["cache_write_tokens"])
	if cacheWriteTokens > 0 {
		return cacheWriteTokens
	}

	cacheCreationTokens5m := positiveTokenValue(other["cache_creation_tokens_5m"])
	cacheCreationTokens1h := positiveTokenValue(other["cache_creation_tokens_1h"])
	if cacheCreationTokens5m > 0 || cacheCreationTokens1h > 0 {
		return cacheCreationTokens5m + cacheCreationTokens1h
	}

	return positiveTokenValue(other["cache_creation_tokens"])
}

func positiveTokenValue(value any) int {
	switch v := value.(type) {
	case int:
		if v > 0 {
			return v
		}
	case int64:
		if v > 0 {
			return int(v)
		}
	case float64:
		if v > 0 {
			return int(v)
		}
	case float32:
		if v > 0 {
			return int(v)
		}
	}
	return 0
}
