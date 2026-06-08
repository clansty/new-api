package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInitChannelMeta_whenPassThroughChannel(t *testing.T) {
	// Given: channel metadata for the pass-through channel.
	c := &gin.Context{}
	c.Set(string(constant.ContextKeyChannelType), constant.ChannelTypePassThrough)
	c.Set(string(constant.ContextKeyChannelSetting), dto.ChannelSettings{})
	info := &RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions}

	// When: relay metadata is initialized.
	info.InitChannelMeta(c)

	// Then: pass-through body and stream options are enabled for this channel.
	require.True(t, info.ChannelSetting.PassThroughBodyEnabled)
	require.True(t, info.SupportStreamOptions)
}
