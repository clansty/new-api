package model

import (
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
)

var channelMemberPollingIndexes sync.Map

func SelectEnabledChannelMembers(channelId int, limit int) ([]ChannelMember, error) {
	if limit <= 0 {
		return nil, nil
	}
	members, err := enabledChannelMembers(channelId)
	if err != nil || len(members) == 0 {
		return members, err
	}
	if limit > len(members) {
		limit = len(members)
	}
	counter := channelMemberPollingIndex(channelId)
	start := int(counter.Add(uint64(limit))-uint64(limit)) % len(members)
	selected := make([]ChannelMember, 0, limit)
	for offset := range limit {
		selected = append(selected, members[(start+offset)%len(members)])
	}
	return selected, nil
}

func enabledChannelMembers(channelId int) ([]ChannelMember, error) {
	if !common.MemoryCacheEnabled {
		var members []ChannelMember
		err := DB.Where("channel_id = ? AND status = ?", channelId, common.ChannelStatusEnabled).
			Order("id asc").Find(&members).Error
		return members, err
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	members := channelMembersByChannelID[channelId]
	enabled := make([]ChannelMember, 0, len(members))
	for _, member := range members {
		if member.Status == common.ChannelStatusEnabled {
			enabled = append(enabled, member)
		}
	}
	return enabled, nil
}

func channelMemberPollingIndex(channelId int) *atomic.Uint64 {
	if value, ok := channelMemberPollingIndexes.Load(channelId); ok {
		return value.(*atomic.Uint64)
	}
	counter := &atomic.Uint64{}
	actual, _ := channelMemberPollingIndexes.LoadOrStore(channelId, counter)
	return actual.(*atomic.Uint64)
}
