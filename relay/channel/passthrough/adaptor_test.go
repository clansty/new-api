package passthrough

import (
	"io"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdaptorGetRequestURL_whenGeminiModelIsMapped(t *testing.T) {
	// Given: a Gemini request path containing the client-facing model name.
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RequestURLPath:  "/v1beta/models/client-model:streamGenerateContent?alt=sse",
		OriginModelName: "client-model",
		RelayFormat:     types.RelayFormatGemini,
		RelayMode:       relayconstant.RelayModeGemini,
		IsStream:        true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://upstream.example",
			UpstreamModelName: "gemini-2.5-pro",
		},
	}

	// When: the upstream request URL is built.
	url, err := adaptor.GetRequestURL(info)

	// Then: the path stays Gemini-native and uses the mapped upstream model.
	require.NoError(t, err)
	require.Equal(t, "https://upstream.example/v1beta/models/gemini-2.5-pro:streamGenerateContent?alt=sse", url)
}

func TestAdaptorGetRequestURL_whenAdvancedOpenAIEntry(t *testing.T) {
	// Given: an advanced pass-through channel receives an OpenAI chat request.
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RequestURLPath: "/v1/chat/completions",
		RelayFormat:    types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeAdvancedPassThrough,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AdvancedOpenAIBaseURL:    "https://openai-upstream.example",
				AdvancedAnthropicBaseURL: "https://anthropic-upstream.example",
			},
		},
	}

	// When: the upstream request URL is built.
	url, err := adaptor.GetRequestURL(info)

	// Then: the OpenAI-compatible upstream is selected.
	require.NoError(t, err)
	require.Equal(t, "https://openai-upstream.example/v1/chat/completions", url)
}

func TestAdaptorGetRequestURL_whenAdvancedClaudeEntry(t *testing.T) {
	// Given: an advanced pass-through channel receives an Anthropic Messages request.
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RequestURLPath: "/v1/messages",
		RelayFormat:    types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeAdvancedPassThrough,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AdvancedOpenAIBaseURL:    "https://openai-upstream.example",
				AdvancedAnthropicBaseURL: "https://anthropic-upstream.example",
			},
		},
	}

	// When: the upstream request URL is built.
	url, err := adaptor.GetRequestURL(info)

	// Then: the Anthropic upstream is selected.
	require.NoError(t, err)
	require.Equal(t, "https://anthropic-upstream.example/v1/messages", url)
}

func TestAdaptorConvertOpenAIResponsesRequest_whenAdvancedResponsesUnsupported(t *testing.T) {
	// Given: an advanced pass-through channel whose OpenAI-compatible upstream does not support Responses.
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIResponses,
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeAdvancedPassThrough,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AdvancedResponsesSupported: false,
			},
		},
	}
	request := dto.OpenAIResponsesRequest{
		Model: "claude-3-5-sonnet",
		Input: []byte(`"hello"`),
	}

	// When: the request is converted for upstream.
	converted, err := adaptor.ConvertOpenAIResponsesRequest(&gin.Context{}, info, request)
	relaycommon.AppendRequestConversionFromRequest(info, converted)

	// Then: it is downgraded to Anthropic Messages.
	require.NoError(t, err)
	require.IsType(t, &dto.ClaudeRequest{}, converted)
	require.Equal(t, relayconstant.RelayModeClaudeMessages, info.RelayMode)
	require.Equal(t, types.RelayFormat(types.RelayFormatClaude), info.GetFinalRequestRelayFormat())
}

func TestAdaptorConvertOpenAIResponsesRequest_whenAdvancedResponsesSupported(t *testing.T) {
	// Given: an advanced pass-through channel whose OpenAI-compatible upstream supports Responses.
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIResponses,
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeAdvancedPassThrough,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AdvancedResponsesSupported: true,
			},
		},
	}
	request := dto.OpenAIResponsesRequest{
		Model: "gpt-4.1",
		Input: []byte(`"hello"`),
	}

	// When: the request is converted for upstream.
	converted, err := adaptor.ConvertOpenAIResponsesRequest(&gin.Context{}, info, request)
	relaycommon.AppendRequestConversionFromRequest(info, converted)

	// Then: the original Responses payload remains pass-through compatible.
	require.NoError(t, err)
	require.IsType(t, dto.OpenAIResponsesRequest{}, converted)
	require.Equal(t, relayconstant.RelayModeResponses, info.RelayMode)
	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.GetFinalRequestRelayFormat())
}

func TestAdaptorConvertOpenAIResponsesRequest_whenAdvancedCompactUnsupported(t *testing.T) {
	// Given: an advanced pass-through channel receives compact without upstream Responses support.
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIResponsesCompaction,
		RelayMode:   relayconstant.RelayModeResponsesCompact,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeAdvancedPassThrough,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AdvancedResponsesSupported: false,
			},
		},
	}

	// When: the request is converted for upstream.
	converted, err := adaptor.ConvertOpenAIResponsesRequest(&gin.Context{}, info, dto.OpenAIResponsesRequest{})

	// Then: compact is rejected instead of returning an incompatible response shape.
	require.ErrorContains(t, err, "requires OpenAI Responses support")
	require.Nil(t, converted)
}

func TestAdaptorConvertGeminiRequest_whenAdvancedPassThrough(t *testing.T) {
	// Given: an advanced pass-through channel receives a Gemini v1beta request.
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatGemini,
		RelayMode:   relayconstant.RelayModeGemini,
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAdvancedPassThrough,
			UpstreamModelName: "gpt-4.1",
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

	// When: the request is converted for upstream.
	converted, err := adaptor.ConvertGeminiRequest(&gin.Context{}, info, request)
	relaycommon.AppendRequestConversionFromRequest(info, converted)

	// Then: it is converted to OpenAI chat/completions.
	require.NoError(t, err)
	openAIRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.Equal(t, "gpt-4.1", openAIRequest.Model)
	require.Len(t, openAIRequest.Messages, 1)
	require.Equal(t, "hello", openAIRequest.Messages[0].StringContent())
	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAI), info.GetFinalRequestRelayFormat())
}

func TestAdaptorGetRequestURL_whenClientCredentialQueryIsPresent(t *testing.T) {
	// Given: a Gemini request authenticated with a client key in the query string.
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RequestURLPath:  "/v1beta/models/gemini-2.5-pro:generateContent?key=client-token&alt=sse",
		OriginModelName: "gemini-2.5-pro",
		RelayFormat:     types.RelayFormatGemini,
		RelayMode:       relayconstant.RelayModeGemini,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://upstream.example",
			UpstreamModelName: "gemini-2.5-pro",
		},
	}

	// When: the upstream request URL is built.
	url, err := adaptor.GetRequestURL(info)

	// Then: client credentials are stripped while protocol query parameters remain.
	require.NoError(t, err)
	require.Equal(t, "https://upstream.example/v1beta/models/gemini-2.5-pro:generateContent?alt=sse", url)
}

func TestAdaptorGetRequestURL_whenRelayFormatIsUnsupported(t *testing.T) {
	// Given: a pass-through channel receives a request format outside its protocol set.
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RequestURLPath: "/v1/images/generations",
		RelayFormat:    types.RelayFormatOpenAIImage,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://upstream.example",
		},
	}

	// When: the upstream URL is requested.
	url, err := adaptor.GetRequestURL(info)

	// Then: no upstream URL is returned, so the payload is not forwarded first.
	require.ErrorContains(t, err, "unsupported pass-through relay format")
	require.Empty(t, url)
}

func TestAdaptorSetupRequestHeader_whenRelayFormats(t *testing.T) {
	testCases := []struct {
		name       string
		format     types.RelayFormat
		wantHeader string
		wantValue  string
	}{
		{name: "openai", format: types.RelayFormatOpenAI, wantHeader: "Authorization", wantValue: "Bearer test-key"},
		{name: "responses", format: types.RelayFormatOpenAIResponses, wantHeader: "Authorization", wantValue: "Bearer test-key"},
		{name: "claude", format: types.RelayFormatClaude, wantHeader: "x-api-key", wantValue: "test-key"},
		{name: "gemini", format: types.RelayFormatGemini, wantHeader: "x-goog-api-key", wantValue: "test-key"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given: a pass-through adaptor and an inbound request.
			adaptor := &Adaptor{}
			c := &gin.Context{Request: httptestRequestWithHeaders()}
			info := &relaycommon.RelayInfo{
				RelayFormat: testCase.format,
				ChannelMeta: &relaycommon.ChannelMeta{
					ApiKey: "test-key",
				},
			}
			header := http.Header{}

			// When: upstream headers are prepared.
			err := adaptor.SetupRequestHeader(c, &header, info)

			// Then: the protocol-native credential header is set.
			require.NoError(t, err)
			require.Equal(t, testCase.wantValue, header.Get(testCase.wantHeader))
		})
	}
}

func TestAdaptorDoRequest_whenRelayFormatIsUnsupported(t *testing.T) {
	// Given: a pass-through adaptor handling a non-supported relay format.
	adaptor := &Adaptor{}
	c := &gin.Context{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatRerank,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://upstream.example",
		},
	}

	// When: the adaptor is asked to send the upstream request.
	resp, err := adaptor.DoRequest(c, info, io.Reader(nil))

	// Then: the request is rejected before any upstream I/O can happen.
	require.Error(t, err)
	require.Nil(t, resp)
	require.Contains(t, err.Error(), "unsupported pass-through relay format")
}

func httptestRequestWithHeaders() *http.Request {
	req, err := http.NewRequest(http.MethodPost, "/v1/messages", nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	return req
}
