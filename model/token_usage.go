package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const tokenUsageLogBatchSize = 500

type tokenUsageLogRow struct {
	Id    int
	Type  int
	Quota int
	Other string
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

func GetTokenLogUsedQuota(tokenId int) (int, error) {
	rows := make([]tokenUsageLogRow, 0, tokenUsageLogBatchSize)
	total := 0
	err := LOG_DB.Model(&Log{}).
		Select("id", "type", "quota", "other").
		Where("token_id = ? AND type IN ?", tokenId, []int{LogTypeConsume, LogTypeRefund}).
		FindInBatches(&rows, tokenUsageLogBatchSize, func(_ *gorm.DB, _ int) error {
			for _, row := range rows {
				quota := row.Quota
				var metadata tokenUsageMetadata
				if common.UnmarshalJsonStr(row.Other, &metadata) == nil && metadata.TokenQuota != nil {
					quota = *metadata.TokenQuota
				}
				if row.Type == LogTypeRefund {
					total -= quota
				} else {
					total += quota
				}
			}
			return nil
		}).Error
	return total, err
}
