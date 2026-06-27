package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"

	"github.com/stretchr/testify/require"
)

func TestChannelType2APIType_whenPassThroughChannel(t *testing.T) {
	// Given: the pass-through channel type.
	channelType := constant.ChannelTypePassThrough

	// When: the channel type is translated to an API type.
	apiType, ok := ChannelType2APIType(channelType)

	// Then: it routes to the pass-through adaptor.
	require.True(t, ok)
	require.Equal(t, constant.APITypePassThrough, apiType)
}

func TestChannelType2APIType_whenAdvancedPassThroughChannel(t *testing.T) {
	// Given: the advanced pass-through channel type.
	channelType := constant.ChannelTypeAdvancedPassThrough

	// When: the channel type is translated to an API type.
	apiType, ok := ChannelType2APIType(channelType)

	// Then: it reuses the pass-through adaptor.
	require.True(t, ok)
	require.Equal(t, constant.APITypePassThrough, apiType)
}
