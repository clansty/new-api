package model

import "fmt"

// TokenQuotaStat 令牌维度的用量聚合数据（按小时分桶）
type TokenQuotaStat struct {
	TokenId   int    `json:"token_id"`
	TokenName string `json:"token_name"`
	Quota     int    `json:"quota"`
	Count     int    `json:"count"`
	CreatedAt int64  `json:"created_at"`
}

// GetTokenQuotaStats 查询按 token_id + 小时分桶的用量统计
// allUsers=true 时不过滤 user_id（管理员查看全局）
func GetTokenQuotaStats(userId int, startTimestamp int64, endTimestamp int64, allUsers bool) ([]*TokenQuotaStat, error) {
	var stats []*TokenQuotaStat
	tx := LOG_DB.Table("logs").
		Select("token_id, token_name, SUM(quota) as quota, COUNT(*) as count, (created_at - created_at % 3600) as created_at").
		Where("type = ?", LogTypeConsume)

	if !allUsers {
		tx = tx.Where("user_id = ?", userId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}

	tx = tx.Group("token_id, token_name, (created_at - created_at % 3600)").
		Order("created_at ASC")

	err := tx.Find(&stats).Error
	return stats, err
}

// ChannelQuotaStat 渠道维度的用量聚合数据（按小时分桶）
type ChannelQuotaStat struct {
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Quota       int    `json:"quota"`
	Count       int    `json:"count"`
	CreatedAt   int64  `json:"created_at"`
}

type CostStat struct {
	ModelName   string  `json:"model_name"`
	ChannelId   int     `json:"channel_id"`
	ChannelName string  `json:"channel_name"`
	Cost        float64 `json:"cost"`
	CreatedAt   int64   `json:"created_at"`
}

func fillCostStatChannelNames(stats []*CostStat) {
	channelIds := make([]int, 0)
	seen := make(map[int]bool)
	for _, stat := range stats {
		if !seen[stat.ChannelId] {
			channelIds = append(channelIds, stat.ChannelId)
			seen[stat.ChannelId] = true
		}
	}

	channelNames := make(map[int]string)
	if len(channelIds) > 0 {
		type channelInfo struct {
			Id   int
			Name string
		}
		var channels []channelInfo
		DB.Table("channels").Select("id, name").Where("id IN ?", channelIds).Find(&channels)
		for _, channel := range channels {
			channelNames[channel.Id] = channel.Name
		}
	}

	for _, stat := range stats {
		if name, ok := channelNames[stat.ChannelId]; ok {
			stat.ChannelName = name
		} else {
			stat.ChannelName = fmt.Sprintf("#%d", stat.ChannelId)
		}
	}
}

func GetCostStats(startTimestamp int64, endTimestamp int64, username string) ([]*CostStat, error) {
	var stats []*CostStat
	tx := LOG_DB.Table("logs").
		Select("model_name, channel_id, SUM(CASE WHEN type = ? THEN -cost ELSE cost END) as cost, (created_at - created_at % 3600) as created_at", LogTypeRefund).
		Where("type IN ? AND cost IS NOT NULL", []int{LogTypeConsume, LogTypeRefund})

	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}

	err := tx.Group("model_name, channel_id, (created_at - created_at % 3600)").
		Order("created_at ASC").
		Find(&stats).Error
	if err != nil {
		return nil, err
	}
	fillCostStatChannelNames(stats)
	return stats, nil
}

// GetChannelQuotaStats 查询按 channel_id + 小时分桶的用量统计（管理员用）
// LOG_DB 和 DB 可能是不同数据库，故分两步查询再合并 channel name
func GetChannelQuotaStats(startTimestamp int64, endTimestamp int64) ([]*ChannelQuotaStat, error) {
	var stats []*ChannelQuotaStat
	tx := LOG_DB.Table("logs").
		Select("channel_id, SUM(quota) as quota, COUNT(*) as count, (created_at - created_at % 3600) as created_at").
		Where("type = ?", LogTypeConsume)

	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}

	tx = tx.Group("channel_id, (created_at - created_at % 3600)").
		Order("created_at ASC")

	if err := tx.Find(&stats).Error; err != nil {
		return nil, err
	}

	// 收集去重的 channel_id
	seen := make(map[int]bool)
	channelIds := make([]int, 0)
	for _, s := range stats {
		if !seen[s.ChannelId] {
			channelIds = append(channelIds, s.ChannelId)
			seen[s.ChannelId] = true
		}
	}

	// 从 DB 批量查询渠道名称
	channelNameMap := make(map[int]string)
	if len(channelIds) > 0 {
		type chInfo struct {
			Id   int
			Name string
		}
		var channels []chInfo
		DB.Table("channels").Select("id, name").Where("id IN ?", channelIds).Find(&channels)
		for _, ch := range channels {
			channelNameMap[ch.Id] = ch.Name
		}
	}

	for _, s := range stats {
		if name, ok := channelNameMap[s.ChannelId]; ok {
			s.ChannelName = name
		} else {
			s.ChannelName = fmt.Sprintf("#%d", s.ChannelId)
		}
	}

	return stats, nil
}
