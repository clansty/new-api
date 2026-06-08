package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestBatchSetChannelCollapse_refreshesChannelCache_whenMemoryCacheEnabled(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))

	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
	})

	channel := model.Channel{
		Name:   "cached-collapse-channel",
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Group:  "default",
		Models: "gpt-4",
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "gpt-4",
		ChannelId: channel.Id,
		Enabled:   true,
	}).Error)

	model.InitChannelCache()
	cachedBeforeUpdate, err := model.CacheGetChannel(channel.Id)
	require.NoError(t, err)
	require.False(t, cachedBeforeUpdate.Collapsed)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/channel/batch/collapse", map[string]any{
		"ids":       []int{channel.Id},
		"collapsed": true,
	}, 1)

	BatchSetChannelCollapse(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success)
	cachedAfterUpdate, err := model.CacheGetChannel(channel.Id)
	require.NoError(t, err)
	require.True(t, cachedAfterUpdate.Collapsed)
}
