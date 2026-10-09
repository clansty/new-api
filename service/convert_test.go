package service

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func testRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta:       &relaycommon.ChannelMeta{},
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{},
	}
}

func TestStreamResponseOpenAI2ClaudeEmitsSignatureBeforeToolUse(t *testing.T) {
	info := testRelayInfo()
	info.RelayFormat = types.RelayFormatClaude
	info.SendResponseCount = 1
	reasoning := "Need to inspect the file."
	opaque := "EqQBCgIYAhIMsignedOpaqueBlob"

	start := &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_123",
		Model: "deepseek-v4-pro",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role:             "assistant",
					ReasoningContent: &reasoning,
				},
			},
		},
	}
	responses := StreamResponseOpenAI2Claude(start, info)
	require.Len(t, responses, 3)
	require.Equal(t, "content_block_start", responses[1].Type)
	require.Equal(t, "thinking", responses[1].ContentBlock.Type)
	require.NotNil(t, responses[1].ContentBlock.Thinking)
	require.Equal(t, "", *responses[1].ContentBlock.Thinking)
	require.Equal(t, "thinking_delta", responses[2].Delta.Type)
	require.Equal(t, reasoning, *responses[2].Delta.Thinking)

	info.SendResponseCount++
	signature := &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_123",
		Model: "deepseek-v4-pro",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ReasoningOpaque: &opaque,
				},
			},
		},
	}
	responses = StreamResponseOpenAI2Claude(signature, info)
	require.Empty(t, responses)

	info.SendResponseCount++
	args := `{"filePath":"/tmp/1.txt"}`
	tool := &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_123",
		Model: "deepseek-v4-pro",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{
						{
							Index: common.GetPointer[int](0),
							ID:    "call_123",
							Type:  "function",
							Function: dto.FunctionResponse{
								Name:      "read",
								Arguments: args,
							},
						},
					},
				},
			},
		},
	}
	responses = StreamResponseOpenAI2Claude(tool, info)
	require.GreaterOrEqual(t, len(responses), 4)
	require.Equal(t, "content_block_delta", responses[0].Type)
	require.Equal(t, "signature_delta", responses[0].Delta.Type)
	require.NotNil(t, responses[0].Delta.Signature)
	require.Equal(t, opaque, *responses[0].Delta.Signature)
	require.Equal(t, "content_block_stop", responses[1].Type)
	require.Equal(t, "content_block_start", responses[2].Type)
	require.Equal(t, "tool_use", responses[2].ContentBlock.Type)
}

func TestStreamResponseOpenAI2Claude_keepsAttemptStateAfterAnotherAttemptFinishes(t *testing.T) {
	base := testRelayInfo()
	base.RelayFormat = types.RelayFormatClaude
	base.SendResponseCount = 1

	finishedAttempt, err := base.CloneForAttempt()
	require.NoError(t, err)
	activeAttempt, err := base.CloneForAttempt()
	require.NoError(t, err)
	finishedAttempt.SendResponseCount = 1
	activeAttempt.SendResponseCount = 1

	reasoning := "first thinking delta"
	firstDelta := &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl-123",
		Model: "mimo-v2.5-free",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ReasoningContent: &reasoning},
		}},
	}
	require.NotEmpty(t, StreamResponseOpenAI2Claude(firstDelta, finishedAttempt))

	finishedAttempt.SendResponseCount++
	finish := &dto.ChatCompletionsStreamResponse{
		Usage: &dto.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
	}
	require.NotEmpty(t, StreamResponseOpenAI2Claude(finish, finishedAttempt))
	require.True(t, finishedAttempt.ClaudeConvertInfo.Done)

	require.NotEmpty(t, StreamResponseOpenAI2Claude(firstDelta, activeAttempt))
}

func TestStreamResponseOpenAI2ClaudeEmitsBlankSignatureBeforeToolUse(t *testing.T) {
	info := testRelayInfo()
	info.RelayFormat = types.RelayFormatClaude
	info.SendResponseCount = 1
	reasoning := "Need to inspect the file."

	start := &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_123",
		Model: "deepseek-v4-pro",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role:             "assistant",
					ReasoningContent: &reasoning,
				},
			},
		},
	}
	responses := StreamResponseOpenAI2Claude(start, info)
	require.Len(t, responses, 3)
	require.Equal(t, "thinking_delta", responses[2].Delta.Type)

	info.SendResponseCount++
	args := `{"filePath":"/tmp/1.txt"}`
	tool := &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_123",
		Model: "deepseek-v4-pro",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{
						{
							Index: common.GetPointer[int](0),
							ID:    "call_123",
							Type:  "function",
							Function: dto.FunctionResponse{
								Name:      "read",
								Arguments: args,
							},
						},
					},
				},
			},
		},
	}
	responses = StreamResponseOpenAI2Claude(tool, info)
	require.GreaterOrEqual(t, len(responses), 4)
	require.Equal(t, "content_block_delta", responses[0].Type)
	require.Equal(t, "signature_delta", responses[0].Delta.Type)
	require.NotNil(t, responses[0].Delta.Signature)
	require.Equal(t, "", *responses[0].Delta.Signature)
	require.Equal(t, "content_block_stop", responses[1].Type)
	require.Equal(t, "content_block_start", responses[2].Type)
	require.Equal(t, "tool_use", responses[2].ContentBlock.Type)
}

func TestStreamResponseOpenAI2ClaudeClosesToolBlockWithoutUsage(t *testing.T) {
	info := testRelayInfo()
	info.RelayFormat = types.RelayFormatClaude
	info.SendResponseCount = 1
	args := `{"filePath":"/tmp/1.txt"}`

	tool := &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_123",
		Model: "gpt-5.5",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{
						{
							Index: common.GetPointer[int](0),
							ID:    "call_123",
							Type:  "function",
							Function: dto.FunctionResponse{
								Name:      "read",
								Arguments: args,
							},
						},
					},
				},
			},
		},
	}
	responses := StreamResponseOpenAI2Claude(tool, info)
	require.NotEmpty(t, responses)
	require.Equal(t, "content_block_start", responses[1].Type)
	require.Equal(t, "tool_use", responses[1].ContentBlock.Type)

	info.SendResponseCount++
	finishReason := "tool_calls"
	stop := &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_123",
		Model: "gpt-5.5",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{FinishReason: &finishReason},
		},
	}
	responses = StreamResponseOpenAI2Claude(stop, info)
	require.NotEmpty(t, responses)
	require.Equal(t, "content_block_stop", responses[0].Type)
	require.Equal(t, "message_stop", responses[len(responses)-1].Type)
}

func TestClaudeToOpenAIRequestPreservesThinkingForToolUse(t *testing.T) {
	thinking := "I need to inspect the file before answering."
	signature := "EqQBCgIYAhIM1xYvopaqueSignature"
	claudeRequest := dto.ClaudeRequest{
		Model: "deepseek-v4-pro",
		Messages: []dto.ClaudeMessage{
			{
				Role: "assistant",
				Content: []dto.ClaudeMediaMessage{
					{
						Type:      "thinking",
						Thinking:  &thinking,
						Signature: &signature,
					},
					{
						Type:  "tool_use",
						Id:    "call_123",
						Name:  "read",
						Input: map[string]any{"filePath": "/tmp/1.txt"},
					},
				},
			},
		},
	}

	openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, testRelayInfo())
	require.NoError(t, err)
	require.Len(t, openAIRequest.Messages, 1)

	message := openAIRequest.Messages[0]
	require.Equal(t, "assistant", message.Role)
	require.Equal(t, thinking, message.GetReasoningContent())
	require.Equal(t, signature, message.GetReasoningOpaque())
	require.Len(t, message.ParseToolCalls(), 1)
	require.JSONEq(t, `{"filePath":"/tmp/1.txt"}`, message.ParseToolCalls()[0].Function.Arguments)
}

func TestClaudeToOpenAIRequestPreservesSignedThinkingContent(t *testing.T) {
	thinking := "Visible reasoning that DeepSeek needs for tool replay."
	signature := "EqQBCgIYAhIMsignedOpaqueBlob"
	claudeRequest := dto.ClaudeRequest{
		Model: "deepseek-v4-pro",
		Messages: []dto.ClaudeMessage{
			{
				Role: "assistant",
				Content: []dto.ClaudeMediaMessage{
					{
						Type:      "thinking",
						Thinking:  &thinking,
						Signature: &signature,
					},
					{
						Type:  "tool_use",
						Id:    "call_456",
						Name:  "lookup",
						Input: map[string]any{"query": "reasoning"},
					},
				},
			},
		},
	}

	openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, testRelayInfo())
	require.NoError(t, err)
	require.Len(t, openAIRequest.Messages, 1)
	require.Equal(t, thinking, openAIRequest.Messages[0].GetReasoningContent())
	require.Equal(t, signature, openAIRequest.Messages[0].GetReasoningOpaque())
	require.Len(t, openAIRequest.Messages[0].ParseToolCalls(), 1)
}

func TestClaudeToOpenAIRequestOmitsEmptyThinkingSignature(t *testing.T) {
	thinking := "Visible reasoning without a provider signature."
	signature := ""
	claudeRequest := dto.ClaudeRequest{
		Model: "gpt-5",
		Messages: []dto.ClaudeMessage{
			{
				Role: "assistant",
				Content: []dto.ClaudeMediaMessage{
					{
						Type:      "thinking",
						Thinking:  &thinking,
						Signature: &signature,
					},
					{
						Type: "text",
						Text: common.GetPointer("final answer"),
					},
				},
			},
		},
	}

	openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, testRelayInfo())
	require.NoError(t, err)
	require.Len(t, openAIRequest.Messages, 1)
	require.Equal(t, thinking, openAIRequest.Messages[0].GetReasoningContent())
	require.Nil(t, openAIRequest.Messages[0].ReasoningOpaque)

	body, err := common.Marshal(openAIRequest)
	require.NoError(t, err)
	require.NotContains(t, string(body), "reasoning_opaque")
}

func TestClaudeToOpenAIRequestSkipsEmptyThinkingOnlyMessage(t *testing.T) {
	claudeRequest := dto.ClaudeRequest{
		Model: "deepseek-v4-pro",
		Messages: []dto.ClaudeMessage{
			{
				Role: "assistant",
				Content: []dto.ClaudeMediaMessage{
					{Type: "thinking"},
				},
			},
		},
	}

	openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, testRelayInfo())
	require.NoError(t, err)
	require.Empty(t, openAIRequest.Messages)
}

func TestClaudeToOpenAIRequestHandlesNilRelayInfo(t *testing.T) {
	claudeRequest := dto.ClaudeRequest{Model: "deepseek-v4-pro"}

	openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, nil)
	require.NoError(t, err)
	require.Equal(t, "deepseek-v4-pro", openAIRequest.Model)
}

func TestClaudeToOpenAIRequestPreservesThinkingWithNilChannelMeta(t *testing.T) {
	thinking := "Need to call a tool."
	claudeRequest := dto.ClaudeRequest{
		Model: "deepseek-v4-pro",
		Messages: []dto.ClaudeMessage{
			{
				Role: "assistant",
				Content: []dto.ClaudeMediaMessage{
					{Type: "thinking", Thinking: &thinking},
					{Type: "tool_use", Id: "call_123", Name: "read"},
				},
			},
		},
	}

	openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, &relaycommon.RelayInfo{})
	require.NoError(t, err)
	require.Len(t, openAIRequest.Messages, 1)
	require.Equal(t, thinking, openAIRequest.Messages[0].GetReasoningContent())
}

func TestClaudeRequestToResponsesRequestMapsReasoningAndMessages(t *testing.T) {
	thinking := "已有推理不应回放到 Responses input"
	budgetTokens := 8192
	maxTokens := uint(8)
	stream := true
	outputFormat := json.RawMessage(`{"type":"json_object"}`)
	metadata := json.RawMessage(`{"user_id":"user_123"}`)
	claudeRequest := dto.ClaudeRequest{
		Model:         "gpt-5",
		System:        []dto.ClaudeMediaMessage{{Type: "text", Text: common.GetPointer("系统提示")}},
		MaxTokens:     &maxTokens,
		StopSequences: []string{"END"},
		Stream:        &stream,
		OutputFormat:  outputFormat,
		Metadata:      metadata,
		Thinking: &dto.Thinking{
			Type:         "enabled",
			BudgetTokens: &budgetTokens,
		},
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: []dto.ClaudeMediaMessage{{Type: "text", Text: common.GetPointer("你好")}}},
			{Role: "assistant", Content: []dto.ClaudeMediaMessage{
				{Type: "thinking", Thinking: &thinking},
				{Type: "text", Text: common.GetPointer("我需要调用工具")},
				{Type: "tool_use", Id: "call_123", Name: "lookup", Input: map[string]any{"query": "new-api"}},
			}},
			{Role: "user", Content: []dto.ClaudeMediaMessage{{Type: "tool_result", ToolUseId: "call_123", Content: "工具结果"}}},
		},
		Tools: []dto.Tool{{
			Name:        "lookup",
			Description: "查询",
			InputSchema: map[string]any{"type": "object"},
		}},
		ToolChoice: dto.ClaudeToolChoice{Type: "tool", Name: "lookup", DisableParallelToolUse: true},
	}

	responsesRequest, err := ClaudeRequestToResponsesRequest(&claudeRequest)
	require.NoError(t, err)
	require.Equal(t, "gpt-5", responsesRequest.Model)
	require.True(t, *responsesRequest.Stream)
	require.Equal(t, uint(16), *responsesRequest.MaxOutputTokens)
	require.NotNil(t, responsesRequest.Reasoning)
	require.Equal(t, "medium", responsesRequest.Reasoning.Effort)
	require.Equal(t, "detailed", responsesRequest.Reasoning.Summary)
	require.Equal(t, "END", responsesRequest.Stop)
	require.JSONEq(t, `"系统提示"`, string(responsesRequest.Instructions))
	require.JSONEq(t, `"user_123"`, string(responsesRequest.User))
	require.JSONEq(t, `{"format":{"type":"json_object"}}`, string(responsesRequest.Text))
	require.JSONEq(t, `{"type":"function","name":"lookup"}`, string(responsesRequest.ToolChoice))
	require.JSONEq(t, `false`, string(responsesRequest.ParallelToolCalls))
	require.JSONEq(t, `[{"type":"function","name":"lookup","description":"查询","parameters":{"type":"object"}}]`, string(responsesRequest.Tools))
	require.JSONEq(t, `[
		{"role":"user","content":[{"type":"input_text","text":"你好"}]},
		{"role":"assistant","content":[{"type":"output_text","text":"我需要调用工具"}]},
		{"type":"function_call","id":"fc_call_123","call_id":"call_123","name":"lookup","arguments":"{\"query\":\"new-api\"}","status":"completed"},
		{"type":"function_call_output","call_id":"call_123","output":"工具结果"}
	]`, string(responsesRequest.Input))
}

func TestClaudeRequestToResponsesRequestMapsMaxEffortToXHigh(t *testing.T) {
	claudeRequest := dto.ClaudeRequest{
		Model:        "gpt-5",
		OutputConfig: json.RawMessage(`{"effort":"max"}`),
		Messages:     []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
	}

	responsesRequest, err := ClaudeRequestToResponsesRequest(&claudeRequest)
	require.NoError(t, err)
	require.NotNil(t, responsesRequest.Reasoning)
	require.Equal(t, "xhigh", responsesRequest.Reasoning.Effort)
}

func TestClaudeRequestToResponsesRequestKeepsToolsAfterJSONRoundTrip(t *testing.T) {
	claudeRequest := dto.ClaudeRequest{
		Model:    "gpt-5",
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
		Tools: []any{
			map[string]any{
				"name":         "lookup",
				"description":  "查询",
				"input_schema": map[string]any{"type": "object"},
			},
		},
	}

	responsesRequest, err := ClaudeRequestToResponsesRequest(&claudeRequest)
	require.NoError(t, err)
	require.JSONEq(t, `[{"type":"function","name":"lookup","description":"查询","parameters":{"type":"object"}}]`, string(responsesRequest.Tools))
}

func TestResponseOpenAI2ClaudePreservesReasoningBeforeToolUse(t *testing.T) {
	reasoning := "Need the file content first."
	opaque := "EqQBCgIYAhIMsignedOpaqueBlob"
	toolCalls := []dto.ToolCallRequest{
		{
			ID:   "call_123",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "read",
				Arguments: `{"filePath":"/tmp/1.txt"}`,
			},
		},
	}
	toolCallsJSON, err := common.Marshal(toolCalls)
	require.NoError(t, err)

	openAIResponse := &dto.OpenAITextResponse{
		Id:    "chatcmpl_123",
		Model: "deepseek-v4-pro",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message: dto.Message{
					Role:             "assistant",
					ReasoningContent: reasoning,
					ReasoningOpaque:  &opaque,
					ToolCalls:        toolCallsJSON,
				},
				FinishReason: "tool_calls",
			},
		},
	}

	claudeResponse := ResponseOpenAI2Claude(openAIResponse, &relaycommon.RelayInfo{})
	require.Len(t, claudeResponse.Content, 2)
	require.Equal(t, "thinking", claudeResponse.Content[0].Type)
	require.NotNil(t, claudeResponse.Content[0].Thinking)
	require.Equal(t, reasoning, *claudeResponse.Content[0].Thinking)
	require.NotNil(t, claudeResponse.Content[0].Signature)
	require.Equal(t, opaque, *claudeResponse.Content[0].Signature)
	require.Equal(t, "tool_use", claudeResponse.Content[1].Type)
	require.Equal(t, "call_123", claudeResponse.Content[1].Id)
	require.Equal(t, "read", claudeResponse.Content[1].Name)
}

func TestClaudeToOpenAIRequestKeepsTextAlongsideToolCalls(t *testing.T) {
	claudeRequest := dto.ClaudeRequest{
		Model: "gpt-5",
		Messages: []dto.ClaudeMessage{
			{
				Role: "assistant",
				Content: []dto.ClaudeMediaMessage{
					{Type: "text", Text: common.GetPointer("我先读一下这个文件")},
					{Type: "tool_use", Id: "call_1", Name: "read", Input: map[string]any{"path": "/tmp/a.txt"}},
				},
			},
		},
	}

	openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, testRelayInfo())
	require.NoError(t, err)
	require.Len(t, openAIRequest.Messages, 1)

	message := openAIRequest.Messages[0]
	require.Equal(t, "我先读一下这个文件", message.StringContent())
	require.Len(t, message.ParseToolCalls(), 1)
	require.Equal(t, "read", message.ParseToolCalls()[0].Function.Name)
}

func TestClaudeToOpenAIRequestAccumulatesInterleavedThinking(t *testing.T) {
	first := "第一步推理。"
	second := "第二步推理。"
	claudeRequest := dto.ClaudeRequest{
		Model: "deepseek-v4-pro",
		Messages: []dto.ClaudeMessage{
			{
				Role: "assistant",
				Content: []dto.ClaudeMediaMessage{
					{Type: "thinking", Thinking: &first},
					{Type: "text", Text: common.GetPointer("继续")},
					{Type: "thinking", Thinking: &second},
				},
			},
		},
	}

	openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, testRelayInfo())
	require.NoError(t, err)
	require.Len(t, openAIRequest.Messages, 1)
	require.Equal(t, "第一步推理。\n第二步推理。", openAIRequest.Messages[0].GetReasoningContent())
}

func TestResponseOpenAI2ClaudeKeepsTextAlongsideToolUse(t *testing.T) {
	response := &dto.OpenAITextResponse{
		Id:    "chatcmpl-1",
		Model: "gpt-5",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index:        0,
				FinishReason: "tool_calls",
				Message: dto.Message{
					Role:    "assistant",
					Content: "我先读一下这个文件",
				},
			},
		},
	}
	response.Choices[0].Message.SetToolCalls([]dto.ToolCallRequest{
		{
			ID:   "call_1",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "read",
				Arguments: `{"path":"/tmp/a.txt"}`,
			},
		},
	})

	converted := ResponseOpenAI2Claude(response, testRelayInfo())

	blocks := converted.Content
	require.Len(t, blocks, 2)
	require.Equal(t, "text", blocks[0].Type)
	require.Equal(t, "我先读一下这个文件", blocks[0].GetText())
	require.Equal(t, "tool_use", blocks[1].Type)
	require.Equal(t, "read", blocks[1].Name)
	require.Equal(t, "tool_use", converted.StopReason)
}

func TestResponseOpenAI2ClaudeKeepsToolUseWhenFinishReasonIsStop(t *testing.T) {
	response := &dto.OpenAITextResponse{
		Id:    "chatcmpl-2",
		Model: "gpt-5",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index:        0,
				FinishReason: "stop",
				Message:      dto.Message{Role: "assistant"},
			},
		},
	}
	response.Choices[0].Message.SetToolCalls([]dto.ToolCallRequest{
		{
			ID:       "call_9",
			Type:     "function",
			Function: dto.FunctionRequest{Name: "search", Arguments: `{"q":"x"}`},
		},
	})

	converted := ResponseOpenAI2Claude(response, testRelayInfo())

	blocks := converted.Content
	require.Len(t, blocks, 1)
	require.Equal(t, "tool_use", blocks[0].Type)
}

func TestClaudeToOpenAIRequestMapsToolChoice(t *testing.T) {
	tools := []any{
		map[string]any{
			"name":         "read",
			"input_schema": map[string]any{"type": "object"},
		},
	}

	cases := []struct {
		name     string
		choice   any
		want     any
		parallel *bool
	}{
		{name: "any to required", choice: map[string]any{"type": "any"}, want: "required"},
		{name: "auto stays auto", choice: map[string]any{"type": "auto"}, want: "auto"},
		{name: "none stays none", choice: map[string]any{"type": "none"}, want: "none"},
		{
			name:   "named tool expands to function selector",
			choice: map[string]any{"type": "tool", "name": "read"},
			want:   map[string]any{"type": "function", "function": map[string]any{"name": "read"}},
		},
		{
			name:     "disable parallel maps to parallel_tool_calls",
			choice:   map[string]any{"type": "auto", "disable_parallel_tool_use": true},
			want:     "auto",
			parallel: common.GetPointer(false),
		},
		{
			name:   "string form",
			choice: "any",
			want:   "required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claudeRequest := dto.ClaudeRequest{
				Model:      "gpt-5",
				Tools:      tools,
				ToolChoice: tc.choice,
			}

			openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, testRelayInfo())
			require.NoError(t, err)
			require.Equal(t, tc.want, openAIRequest.ToolChoice)
			if tc.parallel == nil {
				require.Nil(t, openAIRequest.ParallelTooCalls)
			} else {
				require.NotNil(t, openAIRequest.ParallelTooCalls)
				require.Equal(t, *tc.parallel, *openAIRequest.ParallelTooCalls)
			}
		})
	}
}

func TestClaudeToOpenAIRequestDropsForcedToolChoiceWithoutTools(t *testing.T) {
	claudeRequest := dto.ClaudeRequest{
		Model:      "gpt-5",
		ToolChoice: map[string]any{"type": "tool", "name": "read"},
	}

	openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, testRelayInfo())
	require.NoError(t, err)
	require.Nil(t, openAIRequest.ToolChoice)
}

func TestClaudeToOpenAIRequestKeepsCacheControlOnSingleTextBlock(t *testing.T) {
	claudeRequest := dto.ClaudeRequest{
		Model: "claude-sonnet-4-5",
		Messages: []dto.ClaudeMessage{
			{
				Role: "user",
				Content: []dto.ClaudeMediaMessage{{
					Type:         "text",
					Text:         common.GetPointer("hello"),
					CacheControl: json.RawMessage(`{"type":"ephemeral"}`),
				}},
			},
		},
	}

	openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, testRelayInfo())
	require.NoError(t, err)
	require.Len(t, openAIRequest.Messages, 1)

	// 带缓存断点的块必须保持数组形态，折叠成字符串会把 cache_control 整个丢掉。
	parts := openAIRequest.Messages[0].ParseContent()
	require.Len(t, parts, 1)
	require.Equal(t, "hello", parts[0].Text)
	require.JSONEq(t, `{"type":"ephemeral"}`, string(parts[0].CacheControl))
}

func TestClaudeToOpenAIRequestDropsToolChoiceNoneWithoutTools(t *testing.T) {
	claudeRequest := dto.ClaudeRequest{
		Model:      "gpt-5",
		ToolChoice: map[string]any{"type": "none"},
	}

	openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, testRelayInfo())
	require.NoError(t, err)
	// 严格 OpenAI 兼容上游会拒绝「带 tool_choice 却没有 tools」的请求。
	require.Nil(t, openAIRequest.ToolChoice)
}
