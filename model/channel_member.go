package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type ChannelMember struct {
	Id                 int     `json:"id"`
	ChannelId          int     `json:"channel_id" gorm:"index;not null"`
	Name               string  `json:"name" gorm:"type:varchar(255);not null"`
	Key                string  `json:"-" gorm:"type:text;not null"`
	BaseURL            *string `json:"base_url,omitempty" gorm:"column:base_url;type:text"`
	Proxy              *string `json:"proxy,omitempty" gorm:"type:text"`
	Status             int     `json:"status" gorm:"default:1;index"`
	CreatedTime        int64   `json:"created_time" gorm:"bigint"`
	TestTime           int64   `json:"test_time" gorm:"bigint"`
	ResponseTime       int     `json:"response_time"`
	Balance            float64 `json:"balance"`
	BalanceUpdatedTime int64   `json:"balance_updated_time" gorm:"bigint"`
	DisabledReason     string  `json:"disabled_reason,omitempty" gorm:"type:text"`
	DisabledTime       int64   `json:"disabled_time,omitempty" gorm:"bigint"`
	KeyPreview         string  `json:"key_preview,omitempty" gorm:"-"`
}

func (member *ChannelMember) SetKeyPreview() {
	key := strings.TrimSpace(member.Key)
	if len(key) <= 12 {
		member.KeyPreview = key
		return
	}
	member.KeyPreview = key[:6] + "..." + key[len(key)-4:]
}

func ResolveChannelMember(channel *Channel, member *ChannelMember) *Channel {
	if channel == nil || member == nil {
		return channel
	}
	resolved := *channel
	resolved.Key = member.Key
	resolved.SelectedMemberId = member.Id
	resolved.SelectedMemberName = member.Name
	if member.BaseURL != nil {
		resolved.BaseURL = member.BaseURL
	}
	if member.Proxy != nil {
		setting := resolved.GetSetting()
		setting.Proxy = *member.Proxy
		resolved.SetSetting(setting)
	}
	return &resolved
}

func GetChannelMembers(channelId int, includeKey bool) ([]ChannelMember, error) {
	query := DB.Where("channel_id = ?", channelId).Order("id asc")
	var members []ChannelMember
	if err := query.Find(&members).Error; err != nil {
		return nil, err
	}
	for i := range members {
		members[i].SetKeyPreview()
		if !includeKey {
			members[i].Key = ""
		}
	}
	return members, nil
}

func GetChannelMember(channelId int, memberId int, includeKey bool) (*ChannelMember, error) {
	query := DB.Where("channel_id = ? AND id = ?", channelId, memberId)
	if !includeKey {
		query = query.Omit("key")
	}
	var member ChannelMember
	if err := query.First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func InsertChannelMember(member *ChannelMember) error {
	if member == nil || member.ChannelId <= 0 || member.Key == "" {
		return errors.New("渠道组成员参数无效")
	}
	if member.Status == common.ChannelStatusUnknown {
		member.Status = common.ChannelStatusEnabled
	}
	if member.CreatedTime == 0 {
		member.CreatedTime = common.GetTimestamp()
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(member).Error; err != nil {
			return err
		}
		return syncChannelGroupStatus(tx, member.ChannelId)
	})
}

func InsertChannelGroup(channel *Channel, members []ChannelMember) error {
	if channel == nil || len(members) == 0 {
		return errors.New("渠道组至少需要一个成员")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		channel.IsGroup = true
		channel.Key = ""
		channel.ChannelInfo.IsMultiKey = false
		channel.ChannelInfo.MultiKeySize = 0
		channel.ChannelInfo.MultiKeyStatusList = nil
		if channel.ParallelRequests < 1 {
			channel.ParallelRequests = 1
		}
		if err := tx.Create(channel).Error; err != nil {
			return err
		}
		if err := channel.AddAbilities(tx); err != nil {
			return err
		}
		for i := range members {
			if strings.TrimSpace(members[i].Key) == "" {
				return errors.New("成员密钥不能为空")
			}
			members[i].ChannelId = channel.Id
			if members[i].Name == "" {
				members[i].Name = fmt.Sprintf("成员 %d", i+1)
			}
			if members[i].Status == common.ChannelStatusUnknown {
				members[i].Status = common.ChannelStatusEnabled
			}
			if members[i].CreatedTime == 0 {
				members[i].CreatedTime = common.GetTimestamp()
			}
		}
		return tx.Create(&members).Error
	})
}

func ConvertChannelToGroup(channelId int, parallelRequests int) error {
	if parallelRequests < 1 || parallelRequests > 4 {
		return errors.New("并行请求数必须在 1 到 4 之间")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var channel Channel
		if err := tx.First(&channel, channelId).Error; err != nil {
			return err
		}
		if channel.IsGroup {
			return errors.New("该渠道已经是渠道组")
		}
		keys := channel.GetKeys()
		if len(keys) == 0 {
			return errors.New("渠道密钥不能为空")
		}
		members := make([]ChannelMember, 0, len(keys))
		for i, key := range keys {
			if strings.TrimSpace(key) == "" {
				continue
			}
			members = append(members, ChannelMember{
				ChannelId:   channelId,
				Name:        fmt.Sprintf("成员 %d", i+1),
				Key:         key,
				Status:      common.ChannelStatusEnabled,
				CreatedTime: common.GetTimestamp(),
			})
		}
		if len(members) == 0 {
			return errors.New("渠道密钥不能为空")
		}
		if err := tx.Create(&members).Error; err != nil {
			return err
		}
		return tx.Model(&channel).Updates(map[string]any{
			"is_group":          true,
			"parallel_requests": parallelRequests,
			"key":               "",
			"channel_info":      ChannelInfo{},
		}).Error
	})
}

func UpdateChannelMember(member *ChannelMember, updateKey bool) error {
	if member == nil || member.Id <= 0 || member.ChannelId <= 0 {
		return errors.New("渠道组成员参数无效")
	}
	updates := map[string]any{
		"name":     member.Name,
		"base_url": member.BaseURL,
		"proxy":    member.Proxy,
	}
	if updateKey {
		if member.Key == "" {
			return errors.New("成员密钥不能为空")
		}
		updates["key"] = member.Key
	}
	result := DB.Model(&ChannelMember{}).
		Where("id = ? AND channel_id = ?", member.Id, member.ChannelId).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func DeleteChannelMember(channelId int, memberId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&ChannelMember{}).Where("channel_id = ?", channelId).Count(&count).Error; err != nil {
			return err
		}
		if count <= 1 {
			return errors.New("渠道组至少需要一个成员")
		}
		result := tx.Where("id = ? AND channel_id = ?", memberId, channelId).Delete(&ChannelMember{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return syncChannelGroupStatus(tx, channelId)
	})
}

func UpdateChannelMemberResponseTime(channelId int, memberId int, responseTime int64) {
	err := DB.Model(&ChannelMember{}).
		Where("id = ? AND channel_id = ?", memberId, channelId).
		Updates(map[string]any{
			"test_time":     common.GetTimestamp(),
			"response_time": int(responseTime),
		}).Error
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to update channel member response time: channel_id=%d, member_id=%d, error=%v", channelId, memberId, err))
	}
}

func UpdateChannelMemberStatus(channelId int, memberId int, status int, reason string) (bool, error) {
	changed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var member ChannelMember
		if err := tx.Where("id = ? AND channel_id = ?", memberId, channelId).First(&member).Error; err != nil {
			return err
		}
		if member.Status == status {
			return nil
		}
		updates := map[string]any{"status": status}
		if status == common.ChannelStatusEnabled {
			updates["disabled_reason"] = ""
			updates["disabled_time"] = 0
		} else {
			updates["disabled_reason"] = reason
			updates["disabled_time"] = common.GetTimestamp()
		}
		if err := tx.Model(&member).Updates(updates).Error; err != nil {
			return err
		}

		if err := syncChannelGroupStatus(tx, channelId); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if changed && common.MemoryCacheEnabled {
		InitChannelCache()
	}
	return changed, nil
}

func syncChannelGroupStatus(tx *gorm.DB, channelId int) error {
	var enabledCount int64
	if err := tx.Model(&ChannelMember{}).
		Where("channel_id = ? AND status = ?", channelId, common.ChannelStatusEnabled).
		Count(&enabledCount).Error; err != nil {
		return err
	}
	var channel Channel
	if err := tx.First(&channel, channelId).Error; err != nil {
		return err
	}
	groupStatus := channel.Status
	if enabledCount == 0 && groupStatus == common.ChannelStatusEnabled {
		groupStatus = common.ChannelStatusAutoDisabled
	}
	if enabledCount > 0 && groupStatus == common.ChannelStatusAutoDisabled {
		groupStatus = common.ChannelStatusEnabled
	}
	if groupStatus == channel.Status {
		return nil
	}
	if err := tx.Model(&channel).Update("status", groupStatus).Error; err != nil {
		return err
	}
	if err := tx.Model(&Ability{}).Where("channel_id = ?", channelId).
		Update("enabled", groupStatus == common.ChannelStatusEnabled).Error; err != nil {
		return fmt.Errorf("更新渠道组能力状态: %w", err)
	}
	return nil
}
