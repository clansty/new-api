package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAllChannels_ordersByWeightAndId_whenPriorityMatches(t *testing.T) {
	truncateTables(t)

	lowWeight := uint(10)
	midWeight := uint(20)
	highWeight := uint(30)
	priority := int64(5)
	channels := []Channel{
		testOrderedChannel("low-weight", priority, lowWeight),
		testOrderedChannel("mid-weight-old", priority, midWeight),
		testOrderedChannel("mid-weight-new", priority, midWeight),
		testOrderedChannel("high-weight", priority, highWeight),
	}
	require.NoError(t, DB.Create(&channels).Error)

	got, err := GetAllChannels(0, 10, false, false)

	require.NoError(t, err)
	require.Len(t, got, 4)
	require.Equal(t, []int{channels[3].Id, channels[2].Id, channels[1].Id, channels[0].Id}, channelIDs(got))
}

func testOrderedChannel(name string, priority int64, weight uint) Channel {
	return Channel{
		Name:     name,
		Key:      "test-key",
		Priority: &priority,
		Weight:   &weight,
	}
}

func channelIDs(channels []*Channel) []int {
	ids := make([]int, 0, len(channels))
	for _, channel := range channels {
		ids = append(ids, channel.Id)
	}
	return ids
}
