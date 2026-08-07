package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResolveChannelMember_appliesConnectionOverrides(t *testing.T) {
	baseURL := "https://default.example"
	channel := Channel{
		Id:      7,
		Name:    "共享配置",
		Key:     "",
		BaseURL: &baseURL,
	}
	channel.SetSetting(dto.ChannelSettings{Proxy: "socks5://default:1080"})
	memberBaseURL := "https://member.example"
	memberProxy := "socks5://member:1080"
	member := ChannelMember{
		Id:        11,
		ChannelId: 7,
		Name:      "节点 A",
		Key:       "sk-member",
		BaseURL:   &memberBaseURL,
		Proxy:     &memberProxy,
	}

	resolved := ResolveChannelMember(&channel, &member)

	require.Equal(t, "sk-member", resolved.Key)
	require.Equal(t, memberBaseURL, resolved.GetBaseURL())
	require.Equal(t, memberProxy, resolved.GetSetting().Proxy)
	require.Equal(t, 11, resolved.SelectedMemberId)
	require.Equal(t, "节点 A", resolved.SelectedMemberName)
	require.Equal(t, "共享配置", channel.Name)
}

func TestUpdateChannelMemberStatus_disablesGroupWhenLastMemberFails(t *testing.T) {
	truncateTables(t)
	channel := Channel{
		Name:             "竞速组",
		Status:           common.ChannelStatusEnabled,
		IsGroup:          true,
		ParallelRequests: 2,
	}
	require.NoError(t, DB.Create(&channel).Error)
	members := []ChannelMember{
		{ChannelId: channel.Id, Name: "A", Key: "sk-a", Status: common.ChannelStatusEnabled},
		{ChannelId: channel.Id, Name: "B", Key: "sk-b", Status: common.ChannelStatusAutoDisabled},
	}
	require.NoError(t, DB.Create(&members).Error)

	changed, err := UpdateChannelMemberStatus(channel.Id, members[0].Id, common.ChannelStatusAutoDisabled, "上游认证失败")

	require.NoError(t, err)
	require.True(t, changed)
	var updated Channel
	require.NoError(t, DB.First(&updated, channel.Id).Error)
	require.Equal(t, common.ChannelStatusAutoDisabled, updated.Status)
}

func TestUpdateChannelMemberStatus_enablesGroupWhenMemberRecovers(t *testing.T) {
	truncateTables(t)
	channel := Channel{
		Name:             "恢复组",
		Status:           common.ChannelStatusAutoDisabled,
		IsGroup:          true,
		ParallelRequests: 2,
	}
	require.NoError(t, DB.Create(&channel).Error)
	member := ChannelMember{
		ChannelId: channel.Id,
		Name:      "A",
		Key:       "sk-a",
		Status:    common.ChannelStatusAutoDisabled,
	}
	require.NoError(t, DB.Create(&member).Error)

	changed, err := UpdateChannelMemberStatus(channel.Id, member.Id, common.ChannelStatusEnabled, "")

	require.NoError(t, err)
	require.True(t, changed)
	var updated Channel
	require.NoError(t, DB.First(&updated, channel.Id).Error)
	require.Equal(t, common.ChannelStatusEnabled, updated.Status)
}

func TestSelectRandomChannelMembers_usesRandomIndexesWithoutReplacement(t *testing.T) {
	members := []ChannelMember{{Id: 1}, {Id: 2}, {Id: 3}, {Id: 4}}
	randomIndexes := []int{3, 1}
	call := 0

	selected := selectRandomChannelMembers(members, 2, func(limit int) int {
		require.Less(t, randomIndexes[call], limit)
		index := randomIndexes[call]
		call++
		return index
	})

	require.Equal(t, []int{4, 3}, []int{selected[0].Id, selected[1].Id})
	require.Equal(t, 2, call)
}

func TestSelectEnabledChannelMembers_selectsUniqueEnabledMembers(t *testing.T) {
	truncateTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })
	channel := Channel{Name: "轮转组", Status: common.ChannelStatusEnabled, IsGroup: true, ParallelRequests: 2}
	require.NoError(t, DB.Create(&channel).Error)
	members := []ChannelMember{
		{ChannelId: channel.Id, Name: "A", Key: "sk-a", Status: common.ChannelStatusEnabled},
		{ChannelId: channel.Id, Name: "B", Key: "sk-b", Status: common.ChannelStatusAutoDisabled},
		{ChannelId: channel.Id, Name: "C", Key: "sk-c", Status: common.ChannelStatusEnabled},
		{ChannelId: channel.Id, Name: "D", Key: "sk-d", Status: common.ChannelStatusEnabled},
	}
	require.NoError(t, DB.Create(&members).Error)

	selected, err := SelectEnabledChannelMembers(channel.Id, 2)
	require.NoError(t, err)
	require.Len(t, selected, 2)
	require.NotEqual(t, selected[0].Id, selected[1].Id)
	require.NotEqual(t, members[1].Id, selected[0].Id)
	require.NotEqual(t, members[1].Id, selected[1].Id)
}

func TestConvertChannelToGroup_preservesChannelAndSplitsKeys(t *testing.T) {
	truncateTables(t)
	channel := Channel{
		Name:   "待转换渠道",
		Status: common.ChannelStatusEnabled,
		Key:    "sk-a\nsk-b",
		ChannelInfo: ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 2,
		},
	}
	require.NoError(t, DB.Create(&channel).Error)

	err := ConvertChannelToGroup(channel.Id, 2)

	require.NoError(t, err)
	updated, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.True(t, updated.IsGroup)
	require.Equal(t, 2, updated.ParallelRequests)
	require.Empty(t, updated.Key)
	require.False(t, updated.ChannelInfo.IsMultiKey)
	members, err := GetChannelMembers(channel.Id, true)
	require.NoError(t, err)
	require.Len(t, members, 2)
	require.Equal(t, []string{"sk-a", "sk-b"}, []string{members[0].Key, members[1].Key})
}

func TestDeleteChannelMember_disablesGroupWhenOnlyDisabledMembersRemain(t *testing.T) {
	truncateTables(t)
	channel := Channel{Name: "删除成员组", Status: common.ChannelStatusEnabled, IsGroup: true}
	require.NoError(t, DB.Create(&channel).Error)
	members := []ChannelMember{
		{ChannelId: channel.Id, Name: "可用", Key: "sk-enabled", Status: common.ChannelStatusEnabled},
		{ChannelId: channel.Id, Name: "已禁用", Key: "sk-disabled", Status: common.ChannelStatusAutoDisabled},
	}
	require.NoError(t, DB.Create(&members).Error)

	err := DeleteChannelMember(channel.Id, members[0].Id)

	require.NoError(t, err)
	updated, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, common.ChannelStatusAutoDisabled, updated.Status)
}
