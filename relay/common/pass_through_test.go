package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/stretchr/testify/require"
)

func TestShouldPassThroughRequest_returnsTrueForPassThroughChannel(t *testing.T) {
	// Given: a request routed through the pass-through channel type.
	info := &RelayInfo{
		RelayMode: relayconstant.RelayModeChatCompletions,
		ChannelMeta: &ChannelMeta{
			ChannelType: constant.ChannelTypePassThrough,
		},
	}

	// When: relay code decides whether to reuse the original request body.
	got := ShouldPassThroughRequest(info)

	// Then: pass-through channels always preserve the body.
	require.True(t, got)
}

func TestShouldPassThroughRequest_returnsTrueForChannelBodySetting(t *testing.T) {
	// Given: a normal channel with request body pass-through enabled.
	info := &RelayInfo{
		RelayMode: relayconstant.RelayModeChatCompletions,
		ChannelMeta: &ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
			ChannelSetting: dto.ChannelSettings{
				PassThroughBodyEnabled: true,
			},
		},
	}

	// When: relay code decides whether to reuse the original request body.
	got := ShouldPassThroughRequest(info)

	// Then: the existing channel setting still works.
	require.True(t, got)
}

func TestShouldPassThroughRequest_returnsTrueForClaudeMessages(t *testing.T) {
	// Given: a Claude Messages request routed through the pass-through channel type.
	info := &RelayInfo{
		RelayMode: relayconstant.RelayModeClaudeMessages,
		ChannelMeta: &ChannelMeta{
			ChannelType: constant.ChannelTypePassThrough,
		},
	}

	// When: relay code decides whether to reuse the original request body.
	got := ShouldPassThroughRequest(info)

	// Then: Anthropic-native /v1/messages requests preserve the body.
	require.True(t, got)
}

func TestShouldPassThroughRequest_returnsTrueForAdvancedOpenAIChat(t *testing.T) {
	// Given: an advanced pass-through channel receives an OpenAI chat request.
	info := &RelayInfo{
		RelayMode: relayconstant.RelayModeChatCompletions,
		ChannelMeta: &ChannelMeta{
			ChannelType: constant.ChannelTypeAdvancedPassThrough,
		},
	}

	// When: relay code decides whether to reuse the original request body.
	got := ShouldPassThroughRequest(info)

	// Then: OpenAI chat requests are passed through.
	require.True(t, got)
}

func TestShouldPassThroughRequest_returnsFalseForAdvancedGemini(t *testing.T) {
	// Given: an advanced pass-through channel receives a Gemini v1beta request.
	info := &RelayInfo{
		RelayMode: relayconstant.RelayModeGemini,
		ChannelMeta: &ChannelMeta{
			ChannelType: constant.ChannelTypeAdvancedPassThrough,
		},
	}

	// When: relay code decides whether to reuse the original request body.
	got := ShouldPassThroughRequest(info)

	// Then: Gemini must be converted to OpenAI chat first.
	require.False(t, got)
}

func TestShouldPassThroughRequest_returnsFalseForAdvancedResponsesUnsupported(t *testing.T) {
	// Given: an advanced pass-through channel whose OpenAI upstream does not support Responses.
	info := &RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &ChannelMeta{
			ChannelType: constant.ChannelTypeAdvancedPassThrough,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AdvancedResponsesSupported: false,
			},
		},
	}

	// When: relay code decides whether to reuse the original request body.
	got := ShouldPassThroughRequest(info)

	// Then: Responses must be converted to Anthropic Messages.
	require.False(t, got)
}

func TestShouldPassThroughRequest_ignoresGlobalPassThroughForAdvancedResponsesFallback(t *testing.T) {
	// Given: global pass-through is enabled, but the advanced channel must still convert Responses.
	original := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = true
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = original
	})
	info := &RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &ChannelMeta{
			ChannelType: constant.ChannelTypeAdvancedPassThrough,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AdvancedResponsesSupported: false,
			},
		},
	}

	// When: relay code decides whether to reuse the original request body.
	got := ShouldPassThroughRequest(info)

	// Then: the advanced channel's conversion contract wins over global pass-through.
	require.False(t, got)
}

func TestShouldPassThroughRequest_ignoresGlobalPassThroughForAdvancedGemini(t *testing.T) {
	// Given: global pass-through is enabled, but Gemini v1beta must become OpenAI chat.
	original := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = true
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = original
	})
	info := &RelayInfo{
		RelayMode: relayconstant.RelayModeGemini,
		ChannelMeta: &ChannelMeta{
			ChannelType: constant.ChannelTypeAdvancedPassThrough,
		},
	}

	// When: relay code decides whether to reuse the original request body.
	got := ShouldPassThroughRequest(info)

	// Then: Gemini is not sent as raw pass-through for advanced pass-through channels.
	require.False(t, got)
}

func TestShouldPassThroughRequest_returnsTrueForAdvancedResponsesSupported(t *testing.T) {
	// Given: an advanced pass-through channel whose OpenAI upstream supports Responses.
	info := &RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &ChannelMeta{
			ChannelType: constant.ChannelTypeAdvancedPassThrough,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AdvancedResponsesSupported: true,
			},
		},
	}

	// When: relay code decides whether to reuse the original request body.
	got := ShouldPassThroughRequest(info)

	// Then: Responses requests are passed through to the OpenAI-compatible upstream.
	require.True(t, got)
}

func TestShouldPassThroughRequest_returnsFalseForUnsupportedRelayMode(t *testing.T) {
	// Given: a pass-through channel request for an unsupported endpoint class.
	info := &RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &ChannelMeta{
			ChannelType: constant.ChannelTypePassThrough,
		},
	}

	// When: relay code decides whether to reuse the original request body.
	got := ShouldPassThroughRequest(info)

	// Then: the unsupported endpoint is not sent using pass-through behavior.
	require.False(t, got)
}

func TestBuildPassThroughRequestBody_whenOpenAIModelIsMapped(t *testing.T) {
	// Given: a pass-through OpenAI request body with a client-facing model alias.
	info := &RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		OriginModelName: "alias-model",
		ChannelMeta: &ChannelMeta{
			UpstreamModelName: "upstream-model",
		},
	}
	body := []byte(`{"model":"alias-model","messages":[{"role":"user","content":"hello"}]}`)

	// When: the pass-through body is prepared for upstream.
	got, err := BuildPassThroughRequestBody(info, body)

	// Then: only the model is rewritten to the upstream model name.
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"upstream-model","messages":[{"role":"user","content":"hello"}]}`, string(got))
}

func TestBuildPassThroughRequestBody_whenClaudeModelIsMapped(t *testing.T) {
	// Given: a pass-through Claude request body with a client-facing model alias.
	info := &RelayInfo{
		RelayMode:       relayconstant.RelayModeClaudeMessages,
		OriginModelName: "alias-model",
		ChannelMeta: &ChannelMeta{
			UpstreamModelName: "claude-upstream",
		},
	}
	body := []byte(`{"model":"alias-model","messages":[{"role":"user","content":"hello"}],"max_tokens":128}`)

	// When: the pass-through body is prepared for upstream.
	got, err := BuildPassThroughRequestBody(info, body)

	// Then: only the model is rewritten to the upstream model name.
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"claude-upstream","messages":[{"role":"user","content":"hello"}],"max_tokens":128}`, string(got))
}

func TestBuildPassThroughRequestBody_whenGeminiPathModelIsUsed(t *testing.T) {
	// Given: a Gemini pass-through request whose model is carried by the URL path.
	info := &RelayInfo{
		RelayMode: relayconstant.RelayModeGemini,
		ChannelMeta: &ChannelMeta{
			UpstreamModelName: "gemini-2.5-pro",
		},
	}
	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`)

	// When: the pass-through body is prepared for upstream.
	got, err := BuildPassThroughRequestBody(info, body)

	// Then: the JSON body is preserved because Gemini mapping happens in the path.
	require.NoError(t, err)
	require.JSONEq(t, string(body), string(got))
}
