package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetRandomSatisfiedChannelExcludingFailed_whenPeerHasSamePriority(t *testing.T) {
	withRetrySelectionCache(t, []*Channel{
		testRetryChannel(1, 2, 10),
		testRetryChannel(2, 2, 10),
		testRetryChannel(3, 1, 10),
	})

	channel, err := GetRandomSatisfiedChannelExcludingFailed("default", "gpt-test", map[int]struct{}{1: {}})

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2, channel.Id)
}

func TestGetRandomSatisfiedChannelExcludingFailed_whenSamePriorityExhausted(t *testing.T) {
	withRetrySelectionCache(t, []*Channel{
		testRetryChannel(1, 2, 10),
		testRetryChannel(2, 2, 10),
		testRetryChannel(3, 1, 10),
	})

	channel, err := GetRandomSatisfiedChannelExcludingFailed("default", "gpt-test", map[int]struct{}{
		1: {},
		2: {},
	})

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 3, channel.Id)
}

func TestGetRandomSatisfiedChannelExcludingFailed_whenOnlyLowerPriorityRemains(t *testing.T) {
	withRetrySelectionCache(t, []*Channel{
		testRetryChannel(1, 2, 10),
		testRetryChannel(2, 1, 10),
	})

	channel, err := GetRandomSatisfiedChannelExcludingFailed("default", "gpt-test", map[int]struct{}{1: {}})

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2, channel.Id)
}

func TestGetRandomSatisfiedChannelExcludingFailed_whenSingleChannelAlreadyFailed(t *testing.T) {
	withRetrySelectionCache(t, []*Channel{
		testRetryChannel(1, 2, 10),
	})

	channel, err := GetRandomSatisfiedChannelExcludingFailed("default", "gpt-test", map[int]struct{}{1: {}})

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 1, channel.Id)
}

func TestGetRandomSatisfiedChannelExcludingFailed_whenMemoryCacheDisabled(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)

	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
	common.MemoryCacheEnabled = false

	channelA := testRetryChannel(0, 2, 10)
	channelB := testRetryChannel(0, 2, 10)
	channelC := testRetryChannel(0, 1, 10)
	require.NoError(t, DB.Create(channelA).Error)
	require.NoError(t, DB.Create(channelB).Error)
	require.NoError(t, DB.Create(channelC).Error)
	require.NoError(t, DB.Create(&Ability{Group: "default", Model: "gpt-test", ChannelId: channelA.Id, Enabled: true, Priority: channelA.Priority, Weight: *channelA.Weight}).Error)
	require.NoError(t, DB.Create(&Ability{Group: "default", Model: "gpt-test", ChannelId: channelB.Id, Enabled: true, Priority: channelB.Priority, Weight: *channelB.Weight}).Error)
	require.NoError(t, DB.Create(&Ability{Group: "default", Model: "gpt-test", ChannelId: channelC.Id, Enabled: true, Priority: channelC.Priority, Weight: *channelC.Weight}).Error)

	channel, err := GetRandomSatisfiedChannelExcludingFailed("default", "gpt-test", map[int]struct{}{
		channelA.Id: {},
		channelB.Id: {},
	})

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, channelC.Id, channel.Id)
}

func testRetryChannel(id int, priority int64, weight uint) *Channel {
	return &Channel{
		Id:       id,
		Name:     "retry-test-channel",
		Status:   common.ChannelStatusEnabled,
		Priority: &priority,
		Weight:   &weight,
	}
}

func withRetrySelectionCache(t *testing.T, channels []*Channel) {
	t.Helper()

	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldGroup2Model2Channels := group2model2channels
	oldChannelsIDM := channelsIDM
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		group2model2channels = oldGroup2Model2Channels
		channelsIDM = oldChannelsIDM
	})

	common.MemoryCacheEnabled = true
	group2model2channels = map[string]map[string][]int{
		"default": {
			"gpt-test": make([]int, 0, len(channels)),
		},
	}
	channelsIDM = make(map[int]*Channel, len(channels))
	for _, channel := range channels {
		group2model2channels["default"]["gpt-test"] = append(group2model2channels["default"]["gpt-test"], channel.Id)
		channelsIDM[channel.Id] = channel
	}
}
