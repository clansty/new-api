package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOaiStreamHandler_keepsFinishChunkUsage_whenTrailingUsageDiffers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"id":"chatcmpl-1","model":"deepseek-v4-flash","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-1","model":"deepseek-v4-flash","choices":[{"index":0,"delta":{"content":"42"},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-1","model":"deepseek-v4-flash","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":88,"completion_tokens":15,"total_tokens":103}}`,
		``,
		`data: {"id":"","model":"","choices":[],"usage":{"prompt_tokens":14,"completion_tokens":20,"total_tokens":34}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		RelayMode:   relayconstant.RelayModeChatCompletions,
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "deepseek-v4-flash",
		},
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}

	usage, relayErr := OaiStreamHandler(c, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, 88, usage.PromptTokens)
	require.Equal(t, 15, usage.CompletionTokens)
	require.Equal(t, 103, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), `"output_tokens":15`)
}

func TestOpenaiHandler_whenClineWrapsNonStreamResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"success": true,
			"data": {
				"id": "chatcmpl-1",
				"object": "chat.completion",
				"created": 1,
				"model": "qwen3.7-plus",
				"choices": [{"index": 0, "message": {"role": "assistant", "content": "OK"}, "finish_reason": "stop"}],
				"usage": {"prompt_tokens": 13, "completion_tokens": 32, "total_tokens": 45}
			}
		}`)),
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.cline.bot/api",
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "cline-pass/qwen3.7-plus",
		},
	}

	usage, relayErr := OpenaiHandler(c, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, 13, usage.PromptTokens)
	require.Equal(t, 32, usage.CompletionTokens)
	require.Equal(t, 45, usage.TotalTokens)

	var response dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "chatcmpl-1", response.Id)
	require.Len(t, response.Choices, 1)
	require.Equal(t, "OK", response.Choices[0].Message.StringContent())
}
