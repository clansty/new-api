package model

import (
	"math/rand"

	"github.com/QuantumNous/new-api/common"
)

func SelectEnabledChannelMembers(channelId int, limit int) ([]ChannelMember, error) {
	return SelectEnabledChannelMembersPreferring(channelId, limit, 0)
}

func SelectEnabledChannelMembersPreferring(channelId int, limit int, preferredMemberId int) ([]ChannelMember, error) {
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
	for index := range members {
		if members[index].Id != preferredMemberId {
			continue
		}
		members[0], members[index] = members[index], members[0]
		selectRandomChannelMembers(members[1:], limit-1, rand.Intn)
		return members[:limit], nil
	}
	return selectRandomChannelMembers(members, limit, rand.Intn), nil
}

func selectRandomChannelMembers(members []ChannelMember, limit int, randomIndex func(int) int) []ChannelMember {
	for index := range limit {
		swapIndex := index + randomIndex(len(members)-index)
		members[index], members[swapIndex] = members[swapIndex], members[index]
	}
	return members[:limit]
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
