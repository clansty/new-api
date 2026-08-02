package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
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

func TestValidateChannel_whenAdvancedPassThroughOpenAIBaseURLIsEmpty(t *testing.T) {
	// Given: an advanced pass-through channel without an OpenAI upstream.
	channel := &model.Channel{
		Type: constant.ChannelTypeAdvancedPassThrough,
		Key:  "test-key",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		AdvancedAnthropicBaseURL: "https://anthropic-upstream.example",
	})

	// When: backend channel validation runs.
	err := validateChannel(channel, true)

	// Then: the OpenAI upstream requirement is enforced.
	require.Error(t, err)
	require.Contains(t, err.Error(), "OpenAI 上游地址")
}

func TestValidateChannel_whenAdvancedPassThroughAnthropicBaseURLIsEmpty(t *testing.T) {
	// Given: an advanced pass-through channel without an Anthropic upstream.
	channel := &model.Channel{
		Type: constant.ChannelTypeAdvancedPassThrough,
		Key:  "test-key",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		AdvancedOpenAIBaseURL: "https://openai-upstream.example",
	})

	// When: backend channel validation runs.
	err := validateChannel(channel, true)

	// Then: the Anthropic upstream requirement is enforced.
	require.Error(t, err)
	require.Contains(t, err.Error(), "Anthropic 上游地址")
}

func TestValidateChannel_whenAdvancedPassThroughBaseURLsAreSet(t *testing.T) {
	// Given: an advanced pass-through channel with both upstream protocol URLs.
	channel := &model.Channel{
		Type: constant.ChannelTypeAdvancedPassThrough,
		Key:  "test-key",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		AdvancedOpenAIBaseURL:    "https://openai-upstream.example",
		AdvancedAnthropicBaseURL: "https://anthropic-upstream.example",
	})

	// When: backend channel validation runs.
	err := validateChannel(channel, true)

	// Then: the advanced pass-through-specific validation accepts it.
	require.NoError(t, err)
}

func TestValidateChannel_whenManualUpstreamRateIsNegative(t *testing.T) {
	// Given: 渠道提交了无效的负数上游倍率。
	rate := -0.1
	channel := &model.Channel{ManualUpstreamRateMultiplier: &rate}

	// When: 后端校验渠道。
	err := validateChannel(channel, false)

	// Then: 负数倍率不会进入成本计算。
	require.Error(t, err)
}

func TestValidateChannel_whenResponseTimeoutIsNegative(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{ResponseTimeout: common.GetPointer(-1)}

	err := validateChannel(channel, false)

	require.ErrorContains(t, err, "渠道超时时间不能小于 0")
}
