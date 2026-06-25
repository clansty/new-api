package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

type channelListResponse struct {
	Items []channelListItem `json:"items"`
}

type channelListItem struct {
	ID int `json:"id"`
}

func TestGetAllChannels_ordersByWeightAndId_whenPriorityMatches(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))

	lowWeight := uint(10)
	midWeight := uint(20)
	highWeight := uint(30)
	priority := int64(5)
	channels := []model.Channel{
		testOrderedControllerChannel("low-weight", priority, lowWeight),
		testOrderedControllerChannel("mid-weight-old", priority, midWeight),
		testOrderedControllerChannel("mid-weight-new", priority, midWeight),
		testOrderedControllerChannel("high-weight", priority, highWeight),
	}
	require.NoError(t, db.Create(&channels).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/channel/?p=1&page_size=20", nil, 1)

	GetAllChannels(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data channelListResponse
	require.NoError(t, common.Unmarshal(response.Data, &data))
	require.Equal(t, []int{channels[3].Id, channels[2].Id, channels[1].Id, channels[0].Id}, controllerChannelIDs(data.Items))
}

func testOrderedControllerChannel(name string, priority int64, weight uint) model.Channel {
	return model.Channel{
		Name:     name,
		Key:      "test-key",
		Priority: &priority,
		Weight:   &weight,
	}
}

func controllerChannelIDs(channels []channelListItem) []int {
	ids := make([]int, 0, len(channels))
	for _, channel := range channels {
		ids = append(ids, channel.ID)
	}
	return ids
}
