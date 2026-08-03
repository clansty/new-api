package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type ChannelAffinityForceStatus struct {
	ChannelID     int   `json:"channel_id"`
	ActivatedAt   int64 `json:"activated_at"`
	ForceUntil    int64 `json:"force_until"`
	ScopeCount    int   `json:"scope_count"`
	RemainingSecs int64 `json:"remaining_seconds"`
}

func ActivateChannelAffinityForce(channelID int) (ChannelAffinityForceStatus, error) {
	setting := operation_setting.GetChannelAffinitySetting()
	if setting == nil || !setting.Enabled || len(setting.Rules) == 0 {
		return ChannelAffinityForceStatus{}, fmt.Errorf("渠道亲和性未启用")
	}
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return ChannelAffinityForceStatus{}, err
	}
	if channel.Status != common.ChannelStatusEnabled {
		return ChannelAffinityForceStatus{}, fmt.Errorf("只能对已启用渠道执行强制吸附")
	}

	models := make([]string, 0, len(channel.GetModels()))
	for _, modelName := range channel.GetModels() {
		modelName = strings.TrimSpace(modelName)
		for _, rule := range setting.Rules {
			if modelName != "" && matchAnyRegexCached(rule.ModelRegex, modelName) {
				models = append(models, modelName)
				break
			}
		}
	}
	groups := channel.GetGroups()
	if len(models) == 0 || len(groups) == 0 {
		return ChannelAffinityForceStatus{}, fmt.Errorf("该渠道没有可接管的亲和性分组与模型")
	}

	now := time.Now()
	forceUntil := now.Add(channelAffinityForceDuration)
	scopeCount := 0
	seen := make(map[string]struct{}, len(groups)*len(models))
	for _, group := range groups {
		group = strings.TrimSpace(group)
		for _, modelName := range models {
			key := group + "\x00" + modelName
			if group == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			if err := setChannelAffinityForceRecord(channelAffinityForceRecord{
				ChannelID:         channelID,
				ActivatedAtUnixNs: now.UnixNano(),
				ForceUntilUnix:    forceUntil.Unix(),
				Group:             group,
				Model:             modelName,
			}); err != nil {
				return ChannelAffinityForceStatus{}, err
			}
			scopeCount++
		}
	}
	return ChannelAffinityForceStatus{
		ChannelID:     channelID,
		ActivatedAt:   now.Unix(),
		ForceUntil:    forceUntil.Unix(),
		ScopeCount:    scopeCount,
		RemainingSecs: int64(channelAffinityForceDuration / time.Second),
	}, nil
}

func CancelChannelAffinityForce(channelID int) (int, error) {
	if channelID <= 0 {
		return 0, nil
	}
	records, err := listChannelAffinityForceRecords()
	if err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	cancelled := 0
	for _, record := range records {
		if record.ChannelID != channelID || record.CancelledAtUnix > 0 {
			continue
		}
		record.CancelledAtUnix = now
		record.ForceUntilUnix = now
		if err := setChannelAffinityForceRecord(record); err != nil {
			return cancelled, err
		}
		cancelled++
	}
	return cancelled, nil
}

func ListActiveChannelAffinityForces() ([]ChannelAffinityForceStatus, error) {
	records, err := listChannelAffinityForceRecords()
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	byChannel := make(map[int]ChannelAffinityForceStatus)
	for _, record := range records {
		if record.CancelledAtUnix > 0 || record.ForceUntilUnix <= now {
			continue
		}
		status := byChannel[record.ChannelID]
		status.ChannelID = record.ChannelID
		status.ActivatedAt = record.ActivatedAtUnixNs / int64(time.Second)
		if record.ForceUntilUnix > status.ForceUntil {
			status.ForceUntil = record.ForceUntilUnix
		}
		status.ScopeCount++
		status.RemainingSecs = status.ForceUntil - now
		byChannel[record.ChannelID] = status
	}
	statuses := make([]ChannelAffinityForceStatus, 0, len(byChannel))
	for _, status := range byChannel {
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func getActiveChannelAffinityForce(c *gin.Context, modelName, usingGroup string) (channelAffinityForceRecord, string, bool) {
	groups := []string{usingGroup}
	if usingGroup == "auto" {
		userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		groups = GetUserAutoGroup(userGroup)
	}
	for _, group := range groups {
		record, found := findChannelAffinityForceRecord(group, modelName)
		if !found || record.CancelledAtUnix > 0 || record.ForceUntilUnix <= time.Now().Unix() {
			continue
		}
		channel, err := model.CacheGetChannel(record.ChannelID)
		if err != nil || channel.Status != common.ChannelStatusEnabled || !model.IsChannelEnabledForGroupModel(group, modelName, record.ChannelID) {
			continue
		}
		return record, group, true
	}
	return channelAffinityForceRecord{}, "", false
}

func findChannelAffinityForceRecord(group, modelName string) (channelAffinityForceRecord, bool) {
	record, found, err := getChannelAffinityForceRecord(group, modelName)
	if err == nil && found {
		return record, true
	}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized == "" || normalized == modelName {
		return channelAffinityForceRecord{}, false
	}
	record, found, err = getChannelAffinityForceRecord(group, normalized)
	return record, err == nil && found
}

func shouldRecordChannelAffinityForce(record channelAffinityForceRecord, meta channelAffinityMeta, finalChannelID int, now time.Time) bool {
	if record.CancelledAtUnix > 0 {
		return true
	}
	if meta.SelectedAtUnixNs > 0 && meta.SelectedAtUnixNs < record.ActivatedAtUnixNs {
		return finalChannelID == record.ChannelID
	}
	if meta.ForceActivatedAt == record.ActivatedAtUnixNs && finalChannelID != record.ChannelID {
		return false
	}
	return now.Unix() >= record.ForceUntilUnix || finalChannelID == record.ChannelID
}

func canRecordChannelAffinity(c *gin.Context, meta channelAffinityMeta, finalChannelID int) bool {
	group := meta.UsingGroup
	if group == "auto" {
		group = common.GetContextKeyString(c, constant.ContextKeyAutoGroup)
		if group == "" {
			group = meta.ForceGroup
		}
	}
	record, found := findChannelAffinityForceRecord(group, meta.ModelName)
	if !found {
		return true
	}
	return shouldRecordChannelAffinityForce(record, meta, finalChannelID, time.Now())
}
