package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const tokenUsageMigrationBatchSize = 500

type tokenUsageLogRow struct {
	Id      int
	TokenId int
	Type    int
	Quota   int
	Other   string
}

type tokenUsageMetadata struct {
	TokenQuota *int `json:"token_quota"`
}

func UpdateTokenUsedQuota(id int, quota int) error {
	if id <= 0 || quota == 0 {
		return nil
	}
	if common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeTokenUsedQuota, id, quota)
		return nil
	}
	return updateTokenUsedQuota(id, quota)
}

func updateTokenUsedQuota(id int, quota int) error {
	return DB.Model(&Token{}).Where("id = ?", id).
		Update("used_quota", gorm.Expr("used_quota + ?", quota)).Error
}

func MigrateSubTokenUsedQuota() error {
	lastID := 0
	for {
		var tokens []Token
		if err := DB.Select("id", "used_quota").
			Where("parent_id <> 0 AND used_quota_initialized = ? AND id > ?", false, lastID).
			Order("id").Limit(tokenUsageMigrationBatchSize).Find(&tokens).Error; err != nil {
			return err
		}
		if len(tokens) == 0 {
			return nil
		}

		tokenIDs := make([]int, 0, len(tokens))
		for _, token := range tokens {
			tokenIDs = append(tokenIDs, token.Id)
		}
		usedQuotas, err := getTokenLogUsedQuotas(tokenIDs)
		if err != nil {
			return err
		}
		for _, token := range tokens {
			usedQuota := max(token.UsedQuota, usedQuotas[token.Id], 0)
			if err := DB.Model(&Token{}).
				Where("id = ? AND used_quota_initialized = ?", token.Id, false).
				Updates(map[string]any{
					"used_quota":             gorm.Expr("CASE WHEN used_quota < ? THEN ? ELSE used_quota END", usedQuota, usedQuota),
					"used_quota_initialized": true,
				}).Error; err != nil {
				return err
			}
			lastID = token.Id
		}
	}
}

func getTokenLogUsedQuotas(tokenIDs []int) (map[int]int, error) {
	rows := make([]tokenUsageLogRow, 0, tokenUsageMigrationBatchSize)
	totals := make(map[int]int, len(tokenIDs))
	err := LOG_DB.Model(&Log{}).
		Select("id", "token_id", "type", "quota", "other").
		Where("token_id IN ? AND type IN ?", tokenIDs, []int{LogTypeConsume, LogTypeRefund}).
		FindInBatches(&rows, tokenUsageMigrationBatchSize, func(_ *gorm.DB, _ int) error {
			for _, row := range rows {
				quota := row.Quota
				var metadata tokenUsageMetadata
				if common.UnmarshalJsonStr(row.Other, &metadata) == nil && metadata.TokenQuota != nil {
					quota = *metadata.TokenQuota
				}
				if row.Type == LogTypeRefund {
					totals[row.TokenId] -= quota
				} else {
					totals[row.TokenId] += quota
				}
			}
			return nil
		}).Error
	return totals, err
}
