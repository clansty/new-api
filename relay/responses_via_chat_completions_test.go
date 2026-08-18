package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertResponsesRequestToChatCompletions_preservesCompatibleRequestState(t *testing.T) {
	// Given: 一个包含历史工具调用、多模态输入和结构化输出的 Responses 请求。
	request := &dto.OpenAIResponsesRequest{
		Model:           "gpt-test",
		Instructions:    json.RawMessage(`"保持简洁"`),
		Input:           json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"天气如何"},{"type":"input_image","image_url":"https://example.com/weather.png"}]},{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"city\":\"Shanghai\"}"},{"type":"function_call_output","call_id":"call_1","output":"晴天"}]`),
		MaxOutputTokens: common.GetPointer(uint(256)),
		Temperature:     common.GetPointer(0.4),
		TopP:            common.GetPointer(0.9),
		Metadata:        json.RawMessage(`{"trace_id":"trace_1"}`),
		Store:           json.RawMessage(`false`),
		Reasoning:       &dto.Reasoning{Effort: "medium"},
		Tools:           json.RawMessage(`[{"type":"function","name":"lookup","description":"查询天气","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}]`),
		ToolChoice:      json.RawMessage(`{"type":"function","name":"lookup"}`),
		Text:            json.RawMessage(`{"format":{"type":"json_schema","name":"weather","strict":true,"schema":{"type":"object","properties":{"summary":{"type":"string"}}}}}`),
	}

	// When: 渠道将请求降级到 Chat Completions 协议。
	chatRequest, err := convertResponsesRequestToChatCompletions(request, nil)

	// Then: 所有可表达的请求状态都被保留在 Chat Completions 请求中。
	require.NoError(t, err)
	require.Equal(t, "gpt-test", chatRequest.Model)
	require.Equal(t, uint(256), *chatRequest.MaxTokens)
	require.Equal(t, 0.4, *chatRequest.Temperature)
	require.Equal(t, 0.9, *chatRequest.TopP)
	require.Equal(t, "medium", chatRequest.ReasoningEffort)
	require.JSONEq(t, `{"trace_id":"trace_1"}`, string(chatRequest.Metadata))
	require.JSONEq(t, `false`, string(chatRequest.Store))
	require.Len(t, chatRequest.Messages, 4)
	require.Equal(t, "system", chatRequest.Messages[0].Role)
	require.Equal(t, "保持简洁", chatRequest.Messages[0].StringContent())
	require.Equal(t, "user", chatRequest.Messages[1].Role)
	require.Len(t, chatRequest.Messages[1].ParseContent(), 2)
	require.Equal(t, "assistant", chatRequest.Messages[2].Role)
	require.Equal(t, "lookup", chatRequest.Messages[2].ParseToolCalls()[0].Function.Name)
	require.Equal(t, "tool", chatRequest.Messages[3].Role)
	require.Equal(t, "call_1", chatRequest.Messages[3].ToolCallId)
	require.Len(t, chatRequest.Tools, 1)
	require.Equal(t, "lookup", chatRequest.Tools[0].Function.Name)
	require.JSONEq(t, `{"type":"function","function":{"name":"lookup"}}`, string(mustMarshal(t, chatRequest.ToolChoice)))
	require.NotNil(t, chatRequest.ResponseFormat)
	require.Equal(t, "json_schema", chatRequest.ResponseFormat.Type)
	require.JSONEq(t, `{"name":"weather","strict":true,"schema":{"type":"object","properties":{"summary":{"type":"string"}}}}`, string(chatRequest.ResponseFormat.JsonSchema))
}

func TestConvertResponsesRequestToChatCompletions_preservesReasoningOnlyAssistantTurn(t *testing.T) {
	// Given: 上一轮 Responses 只产出了推理内容，随后用户继续提问。
	request := &dto.OpenAIResponsesRequest{
		Model: "deepseek-v4-flash",
		Input: json.RawMessage(`[
			{"type":"reasoning","summary":[{"type":"summary_text","text":"上一轮仍在分析。"}]},
			{"role":"user","content":[{"type":"input_text","text":"继续"}]}
		]`),
	}

	// When: 请求被降级到 Chat Completions 协议。
	chatRequest, err := convertResponsesRequestToChatCompletions(request, nil)

	// Then: 独立的 assistant 推理轮仍通过 reasoning_content 回传给上游。
	require.NoError(t, err)
	require.Len(t, chatRequest.Messages, 2)
	require.Equal(t, "assistant", chatRequest.Messages[0].Role)
	require.Equal(t, "上一轮仍在分析。", chatRequest.Messages[0].ReasoningContent)
	require.Equal(t, "user", chatRequest.Messages[1].Role)
	require.Equal(t, "继续", chatRequest.Messages[1].ParseContent()[0].Text)
}

func TestConvertResponsesRequestToChatCompletions_rejectsStateWithoutChatEquivalent(t *testing.T) {
	// Given: Chat Completions 无法表达的服务端会话状态。
	request := &dto.OpenAIResponsesRequest{
		Model:              "gpt-test",
		Input:              json.RawMessage(`"hello"`),
		PreviousResponseID: "resp_previous",
	}

	// When: 渠道尝试降级请求。
	_, err := convertResponsesRequestToChatCompletions(request, nil)

	// Then: 返回明确错误而不是丢弃状态后继续请求。
	require.ErrorContains(t, err, "previous_response_id")
}

func TestConvertResponsesRequestToChatCompletions_ignoresIncludeWithoutChatEquivalent(t *testing.T) {
	// Given: Responses 客户端请求只影响附加响应字段的 include。
	request := &dto.OpenAIResponsesRequest{
		Model:   "gpt-test",
		Input:   json.RawMessage(`"hello"`),
		Include: json.RawMessage(`["reasoning.encrypted_content"]`),
	}

	// When: 渠道将请求降级到 Chat Completions 协议。
	chatRequest, err := convertResponsesRequestToChatCompletions(request, nil)

	// Then: include 被静默丢弃，不阻断主要生成请求。
	require.NoError(t, err)
	encoded, err := common.Marshal(chatRequest)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), `"include"`)
}

func TestChatCompletionsResponsesState_emitsResponsesEventsForTextAndToolCalls(t *testing.T) {
	// Given: Chat Completions 上游先输出文本，再输出函数调用及用量。
	state := newChatCompletionsResponsesState("gpt-test")
	content := "你好"
	arguments := `{"city":"Shanghai"}`
	finishReason := "tool_calls"
	toolIndex := 0
	events := state.Handle(&dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_1",
		Created: 1700000000,
		Model:   "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				Role:    "assistant",
				Content: &content,
				ToolCalls: []dto.ToolCallResponse{{
					Index: &toolIndex,
					ID:    "call_1",
					Type:  "function",
					Function: dto.FunctionResponse{
						Name:      "lookup",
						Arguments: arguments,
					},
				}},
			},
			FinishReason: &finishReason,
		}},
		Usage: &dto.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	})
	events = append(events, state.FinalEvents()...)

	// When: 上游 Chat 流被转换为 Responses 事件。
	eventTypes := make([]string, 0, len(events))
	for _, event := range events {
		eventTypes = append(eventTypes, event.Type)
	}

	// Then: 客户端得到完整的生命周期、文本和函数调用事件。
	require.Contains(t, eventTypes, "response.created")
	require.Contains(t, eventTypes, "response.in_progress")
	require.Contains(t, eventTypes, "response.output_text.delta")
	require.Contains(t, eventTypes, "response.function_call_arguments.delta")
	require.Contains(t, eventTypes, "response.completed")
	require.Equal(t, 2, countResponsesEvent(eventTypes, "response.output_item.done"))
	completed := events[len(events)-1]
	require.NotNil(t, completed.Response)
	require.JSONEq(t, `"completed"`, string(completed.Response.Status))
	require.Len(t, completed.Response.Output, 2)
	require.Equal(t, "message", completed.Response.Output[0].Type)
	require.Equal(t, "你好", completed.Response.Output[0].Content[0].Text)
	require.Equal(t, "function_call", completed.Response.Output[1].Type)
	require.Equal(t, "lookup", completed.Response.Output[1].Name)
	require.Equal(t, 15, completed.Response.Usage.TotalTokens)
}

func TestChatCompletionsResponsesStatePlacesReasoningBeforeTextInCombinedDelta(t *testing.T) {
	state := newChatCompletionsResponsesState("deepseek-v4-flash")
	content := "答案"
	reasoning := "先分析问题。"
	finishReason := "stop"
	events := state.Handle(&dto.ChatCompletionsStreamResponse{
		Id: "chatcmpl_1",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				Role:             "assistant",
				Content:          &content,
				ReasoningContent: &reasoning,
			},
			FinishReason: &finishReason,
		}},
	})
	events = append(events, state.FinalEvents()...)

	completed := events[len(events)-1]
	require.NotNil(t, completed.Response)
	require.Len(t, completed.Response.Output, 2)
	require.Equal(t, "reasoning", completed.Response.Output[0].Type)
	require.Equal(t, "message", completed.Response.Output[1].Type)
	for _, event := range events {
		if event.Type == "response.output_item.added" && event.Item != nil && event.Item.Type == "reasoning" {
			require.Len(t, event.Item.Summary, 1)
			require.Equal(t, "summary_text", event.Item.Summary[0].Type)
		}
		if event.Type == "response.output_item.added" && event.Item != nil && event.Item.Type == "message" {
			require.Len(t, event.Item.Content, 1)
			require.Equal(t, "output_text", event.Item.Content[0].Type)
		}
	}
	reasoningDoneIndex := -1
	messageAddedIndex := -1
	for index, event := range events {
		if event.Type == "response.output_item.done" && event.Item != nil && event.Item.Type == "reasoning" {
			reasoningDoneIndex = index
		}
		if event.Type == "response.output_item.added" && event.Item != nil && event.Item.Type == "message" {
			messageAddedIndex = index
		}
	}
	require.NotEqual(t, -1, reasoningDoneIndex)
	require.NotEqual(t, -1, messageAddedIndex)
	require.Less(t, reasoningDoneIndex, messageAddedIndex)
}

func TestChatCompletionsResponseAsResponsesPreservesReasoningContent(t *testing.T) {
	response := &dto.OpenAITextResponse{
		Model: "deepseek-v4-flash",
		Choices: []dto.OpenAITextResponseChoice{{
			Index: 0,
			Message: dto.Message{
				Role:             "assistant",
				Content:          "答案",
				ReasoningContent: "先分析问题。",
			},
			FinishReason: "stop",
		}},
	}

	converted, _ := chatCompletionsResponseAsResponses(response)

	require.Len(t, converted.Output, 2)
	require.Equal(t, "reasoning", converted.Output[0].Type)
	require.Equal(t, "先分析问题。", converted.Output[0].Summary[0].Text)
	require.Equal(t, "message", converted.Output[1].Type)
	require.Equal(t, "答案", converted.Output[1].Content[0].Text)
}

func TestResponsesViaChatCompletions_forwardsConvertedRequestAndReturnsResponses(t *testing.T) {
	// Given: 只支持 Chat Completions 的 OpenAI 兼容上游。
	service.InitHttpClient()
	upstreamRequests := make(chan struct {
		path    string
		request dto.GeneralOpenAIRequest
		err     error
	}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var forwarded dto.GeneralOpenAIRequest
		err := common.DecodeJson(request.Body, &forwarded)
		upstreamRequests <- struct {
			path    string
			request dto.GeneralOpenAIRequest
			err     error
		}{path: request.URL.Path, request: forwarded, err: err}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl_1","object":"chat.completion","created":1700000000,"model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":"晴天"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`))
	}))
	defer upstream.Close()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "http://gateway.test/v1/responses", strings.NewReader(`{}`))
	context.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeResponses,
		RelayFormat:     types.RelayFormatOpenAIResponses,
		OriginModelName: "gpt-test",
		RequestURLPath:  "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			ApiType:           constant.APITypeOpenAI,
			ChannelBaseUrl:    upstream.URL,
			ApiKey:            "test-key",
			UpstreamModelName: "gpt-test",
		},
	}
	adaptor := &openai.Adaptor{}
	adaptor.Init(info)

	// When: 接收到 Responses 请求后，网关请求 Chat Completions 上游。
	usage, apiErr := responsesViaChatCompletions(context, info, adaptor, &dto.OpenAIResponsesRequest{
		Model:           "gpt-test",
		Input:           json.RawMessage(`"天气如何"`),
		Include:         json.RawMessage(`["reasoning.encrypted_content"]`),
		Temperature:     common.GetPointer(0.2),
		MaxOutputTokens: common.GetPointer(uint(128)),
	})

	// Then: 上游收到 Chat Completions 路径和消息，客户端仍得到 Responses 协议响应。
	require.Nil(t, apiErr)
	require.Equal(t, 12, usage.TotalTokens)
	forwarded := <-upstreamRequests
	require.NoError(t, forwarded.err)
	require.Equal(t, "/v1/chat/completions", forwarded.path)
	require.Equal(t, "gpt-test", forwarded.request.Model)
	require.Len(t, forwarded.request.Messages, 1)
	require.Equal(t, "user", forwarded.request.Messages[0].Role)
	require.Equal(t, "天气如何", forwarded.request.Messages[0].StringContent())
	require.Equal(t, 128, int(*forwarded.request.MaxTokens))
	require.Equal(t, 0.2, *forwarded.request.Temperature)
	require.Equal(t, relayconstant.RelayModeResponses, info.RelayMode)
	require.Equal(t, "/v1/responses", info.RequestURLPath)

	var response dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "response", response.Object)
	require.JSONEq(t, `"completed"`, string(response.Status))
	require.Len(t, response.Output, 1)
	require.Equal(t, "晴天", response.Output[0].Content[0].Text)
}

func TestResponsesViaChatCompletions_convertsChatStreamToResponsesEvents(t *testing.T) {
	// Given: 上游仅以 Chat Completions SSE 返回文本和工具调用。
	previousStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = previousStreamingTimeout })
	service.InitHttpClient()
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/v1/chat/completions", request.URL.Path)
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: {\"id\":\"chatcmpl_stream\",\"created\":1700000000,\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"你好\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{\\\"city\\\":\\\"Shanghai\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}}\n\n"))
		_, _ = writer.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "http://gateway.test/v1/responses", strings.NewReader(`{}`))
	context.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeResponses,
		RelayFormat:     types.RelayFormatOpenAIResponses,
		OriginModelName: "gpt-test",
		RequestURLPath:  "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			ApiType:           constant.APITypeOpenAI,
			ChannelBaseUrl:    upstream.URL,
			ApiKey:            "test-key",
			UpstreamModelName: "gpt-test",
		},
	}
	adaptor := &openai.Adaptor{}
	adaptor.Init(info)

	// When: 客户端以 Responses 流式协议发起请求。
	usage, apiErr := responsesViaChatCompletions(context, info, adaptor, &dto.OpenAIResponsesRequest{
		Model:  "gpt-test",
		Input:  json.RawMessage(`"天气如何"`),
		Stream: common.GetPointer(true),
	})

	// Then: 客户端收到完整的 Responses SSE 生命周期。
	require.Nil(t, apiErr)
	require.Equal(t, 15, usage.TotalTokens)
	require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	body := recorder.Body.String()
	require.Contains(t, body, "event: response.output_text.delta")
	require.Contains(t, body, "event: response.function_call_arguments.delta")
	require.Contains(t, body, "event: response.completed")
	require.Contains(t, body, "\"status\":\"completed\"")
}

func countResponsesEvent(events []string, expected string) int {
	count := 0
	for _, event := range events {
		if event == expected {
			count++
		}
	}
	return count
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := common.Marshal(value)
	require.NoError(t, err)
	return data
}
