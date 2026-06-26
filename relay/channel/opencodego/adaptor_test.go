package opencodego

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdaptorGetRequestURL_whenModelUsesMessagesEndpoint(t *testing.T) {
	testCases := []string{
		"minimax-m2.7",
		"qwen3.7-max",
	}

	for _, model := range testCases {
		t.Run(model, func(t *testing.T) {
			adaptor := &Adaptor{}
			info := &relaycommon.RelayInfo{
				RequestURLPath: "/v1/chat/completions",
				RelayFormat:    types.RelayFormatOpenAI,
				RelayMode:      relayconstant.RelayModeChatCompletions,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl:    "https://opencode.ai/zen/go",
					UpstreamModelName: model,
				},
			}

			url, err := adaptor.GetRequestURL(info)

			require.NoError(t, err)
			require.Equal(t, "https://opencode.ai/zen/go/v1/messages", url)
		})
	}
}

func TestAdaptorGetRequestURL_whenModelUsesChatCompletionsEndpoint(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RequestURLPath: "/v1/chat/completions",
		RelayFormat:    types.RelayFormatOpenAI,
		RelayMode:      relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://opencode.ai/zen/go",
			UpstreamModelName: "glm-5.2",
		},
	}

	url, err := adaptor.GetRequestURL(info)

	require.NoError(t, err)
	require.Equal(t, "https://opencode.ai/zen/go/v1/chat/completions", url)
}

func TestAdaptorConvertOpenAIRequest_whenModelUsesChatCompletionsPreservesStreamOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		RelayMode:   relayconstant.RelayModeChatCompletions,
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          constant.ChannelTypeOpenCodeGo,
			UpstreamModelName:    "glm-5.2",
			SupportStreamOptions: true,
		},
	}
	request := &dto.GeneralOpenAIRequest{
		Model: "glm-5.2",
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hello",
			},
		},
		StreamOptions: &dto.StreamOptions{
			IncludeUsage: true,
		},
	}

	converted, err := adaptor.ConvertOpenAIRequest(
		gin.CreateTestContextOnly(nil, gin.New()),
		info,
		request,
	)

	require.NoError(t, err)
	openAIRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.NotNil(t, openAIRequest.StreamOptions)
	require.True(t, openAIRequest.StreamOptions.IncludeUsage)
	require.Equal(t, constant.ChannelTypeOpenCodeGo, info.ChannelType)
}

func TestAdaptorConvertOpenAIRequest_whenModelUsesMessagesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		RelayMode:   relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "qwen3.7-max",
		},
	}
	request := &dto.GeneralOpenAIRequest{
		Model: "qwen3.7-max",
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hello",
			},
		},
	}

	converted, err := adaptor.ConvertOpenAIRequest(
		gin.CreateTestContextOnly(nil, gin.New()),
		info,
		request,
	)

	require.NoError(t, err)
	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	require.Equal(t, "qwen3.7-max", claudeRequest.Model)
	require.Len(t, claudeRequest.Messages, 1)
	require.Equal(t, "user", claudeRequest.Messages[0].Role)
}

func TestAdaptorConvertGeminiRequest_whenModelUsesMessagesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatGemini,
		RelayMode:       relayconstant.RelayModeGemini,
		OriginModelName: "qwen3.7-max",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "qwen3.7-max",
		},
	}
	request := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role: "user",
				Parts: []dto.GeminiPart{
					{Text: "hello"},
				},
			},
		},
	}

	converted, err := adaptor.ConvertGeminiRequest(
		gin.CreateTestContextOnly(nil, gin.New()),
		info,
		request,
	)

	require.NoError(t, err)
	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	require.Equal(t, "qwen3.7-max", claudeRequest.Model)
	require.Len(t, claudeRequest.Messages, 1)
	require.Equal(t, "user", claudeRequest.Messages[0].Role)
}
