package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestBatchSetChannelCollapse_whenIdsProvided(t *testing.T) {
	truncateTables(t)

	channelOne := Channel{
		Name:   "collapsed-channel-1",
		Key:    "test-key-1",
		Status: common.ChannelStatusEnabled,
	}
	channelTwo := Channel{
		Name:   "collapsed-channel-2",
		Key:    "test-key-2",
		Status: common.ChannelStatusAutoDisabled,
	}
	channelThree := Channel{
		Name:   "visible-channel",
		Key:    "test-key-3",
		Status: common.ChannelStatusEnabled,
	}
	require.NoError(t, DB.Create(&channelOne).Error)
	require.NoError(t, DB.Create(&channelTwo).Error)
	require.NoError(t, DB.Create(&channelThree).Error)

	err := BatchSetChannelCollapse([]int{channelOne.Id, channelTwo.Id}, true)

	require.NoError(t, err)
	var channels []Channel
	require.NoError(t, DB.Order("id asc").Find(&channels).Error)
	require.Len(t, channels, 3)
	require.True(t, channels[0].Collapsed)
	require.True(t, channels[1].Collapsed)
	require.False(t, channels[2].Collapsed)
}

func TestBatchSetChannelCollapse_whenSettingFalse(t *testing.T) {
	truncateTables(t)

	channel := Channel{
		Name:      "expanded-channel",
		Key:       "test-key",
		Status:    common.ChannelStatusEnabled,
		Collapsed: true,
	}
	require.NoError(t, DB.Create(&channel).Error)

	err := BatchSetChannelCollapse([]int{channel.Id}, false)

	require.NoError(t, err)
	var updated Channel
	require.NoError(t, DB.First(&updated, "id = ?", channel.Id).Error)
	require.False(t, updated.Collapsed)
}

func TestChannelGetAutoRecover_whenUnset(t *testing.T) {
	channel := Channel{}

	require.True(t, channel.GetAutoRecover())
}

func TestBatchSetChannelAutoRecover_whenSettingFalse(t *testing.T) {
	truncateTables(t)

	channel := Channel{
		Name:        "auto-recover-channel",
		Key:         "test-key",
		Status:      common.ChannelStatusAutoDisabled,
		AutoRecover: common.GetPointer(1),
	}
	require.NoError(t, DB.Create(&channel).Error)

	err := BatchSetChannelAutoRecover([]int{channel.Id}, false)

	require.NoError(t, err)
	var updated Channel
	require.NoError(t, DB.First(&updated, "id = ?", channel.Id).Error)
	require.False(t, updated.GetAutoRecover())
}

func TestBatchSetChannelAutoRecover_whenSettingTrue(t *testing.T) {
	truncateTables(t)

	channel := Channel{
		Name:        "auto-recover-disabled-channel",
		Key:         "test-key",
		Status:      common.ChannelStatusAutoDisabled,
		AutoRecover: common.GetPointer(0),
	}
	require.NoError(t, DB.Create(&channel).Error)

	err := BatchSetChannelAutoRecover([]int{channel.Id}, true)

	require.NoError(t, err)
	var updated Channel
	require.NoError(t, DB.First(&updated, "id = ?", channel.Id).Error)
	require.True(t, updated.GetAutoRecover())
}
