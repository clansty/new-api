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

	channel, err := GetRandomSatisfiedChannelExcludingFailed("default", "gpt-test", map[int]struct{}{1: {}}, false)

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
	}, false)

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 3, channel.Id)
}

func TestGetRandomSatisfiedChannelExcludingFailed_whenOnlyLowerPriorityRemains(t *testing.T) {
	withRetrySelectionCache(t, []*Channel{
		testRetryChannel(1, 2, 10),
		testRetryChannel(2, 1, 10),
	})

	channel, err := GetRandomSatisfiedChannelExcludingFailed("default", "gpt-test", map[int]struct{}{1: {}}, false)

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2, channel.Id)
}

func TestGetRandomSatisfiedChannelExcludingFailed_whenSingleChannelAlreadyFailed(t *testing.T) {
	withRetrySelectionCache(t, []*Channel{
		testRetryChannel(1, 2, 10),
	})

	channel, err := GetRandomSatisfiedChannelExcludingFailed("default", "gpt-test", map[int]struct{}{1: {}}, false)

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 1, channel.Id)
}

func TestGetRandomSatisfiedChannelExcludingFailedStrict_whenNoOtherChannelExists(t *testing.T) {
	withRetrySelectionCache(t, []*Channel{
		testRetryChannel(1, 2, 10),
	})

	channel, err := GetRandomSatisfiedChannelExcludingFailedStrict("default", "gpt-test", map[int]struct{}{1: {}}, false)

	require.NoError(t, err)
	require.Nil(t, channel)
}

func TestGetRandomSatisfiedChannelExcludingFailedStrict_whenOtherChannelExists(t *testing.T) {
	withRetrySelectionCache(t, []*Channel{
		testRetryChannel(1, 2, 10),
		testRetryChannel(2, 2, 10),
	})

	channel, err := GetRandomSatisfiedChannelExcludingFailedStrict("default", "gpt-test", map[int]struct{}{1: {}}, false)

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2, channel.Id)
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
	}, false)

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, channelC.Id, channel.Id)
}

// 下一优先级策略：本优先级只要有渠道失败，就整层跳过、直接尝试下一优先级
func TestGetRandomSatisfiedChannelExcludingFailed_nextPriority_skipsSamePriorityPeer(t *testing.T) {
	withRetrySelectionCache(t, []*Channel{
		testRetryChannel(1, 2, 10),
		testRetryChannel(2, 2, 10),
		testRetryChannel(3, 1, 10),
	})

	// 同优先级模式下会命中 #2；下一优先级模式应跳过 #2，直接命中低优先级 #3
	channel, err := GetRandomSatisfiedChannelExcludingFailed("default", "gpt-test", map[int]struct{}{1: {}}, true)

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 3, channel.Id)
}

// 下一优先级策略：所有优先级层都已有失败时，回退到最高优先级层
func TestGetRandomSatisfiedChannelExcludingFailed_nextPriority_fallbackWhenAllTiersTouched(t *testing.T) {
	withRetrySelectionCache(t, []*Channel{
		testRetryChannel(1, 2, 10),
		testRetryChannel(2, 1, 10),
	})

	channel, err := GetRandomSatisfiedChannelExcludingFailed("default", "gpt-test", map[int]struct{}{
		1: {},
		2: {},
	}, true)

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 1, channel.Id)
}

// 下一优先级策略：无整层未失败的层时，退回到"任意未失败渠道"，而非重试已失败的高优先级渠道
func TestGetRandomSatisfiedChannelExcludingFailed_nextPriority_degradesToUnfailedPeer(t *testing.T) {
	withRetrySelectionCache(t, []*Channel{
		testRetryChannel(1, 2, 10),
		testRetryChannel(2, 1, 10),
		testRetryChannel(3, 1, 10),
	})

	// #1(高优先级) 与 #2(低优先级) 均已失败；低优先级层还剩未失败的 #3，应命中 #3 而非重试 #1
	channel, err := GetRandomSatisfiedChannelExcludingFailed("default", "gpt-test", map[int]struct{}{
		1: {},
		2: {},
	}, true)

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 3, channel.Id)
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
