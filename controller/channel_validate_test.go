package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
)

func TestValidateChannel_whenPassThroughBaseURLIsEmpty(t *testing.T) {
	// Given: a pass-through channel without an upstream base URL.
	channel := &model.Channel{
		Type: constant.ChannelTypePassThrough,
		Key:  "test-key",
	}

	// When: backend channel validation runs.
	err := validateChannel(channel, true)

	// Then: API callers get the same requirement enforced as the UI.
	require.Error(t, err)
	require.Contains(t, err.Error(), "透传渠道必须设置上游地址")
}

func TestValidateChannel_whenPassThroughBaseURLIsSet(t *testing.T) {
	// Given: a pass-through channel with an upstream base URL.
	channel := &model.Channel{
		Type:    constant.ChannelTypePassThrough,
		Key:     "test-key",
		BaseURL: common.GetPointer("https://upstream.example"),
	}

	// When: backend channel validation runs.
	err := validateChannel(channel, true)

	// Then: the pass-through-specific validation accepts it.
	require.NoError(t, err)
}
