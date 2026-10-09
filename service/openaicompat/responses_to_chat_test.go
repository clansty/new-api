package openaicompat

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponsesResponseToChatCompletionsResponsePreservesReasoningContent(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		Model: "deepseek-v4-flash",
		Output: []dto.ResponsesOutput{
			{
				Type: "reasoning",
				Summary: []dto.ResponsesReasoningSummaryPart{{
					Type: "summary_text",
					Text: "先分析问题。",
				}},
			},
			{
				Type:    "message",
				Role:    "assistant",
				Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: "答案"}},
			},
		},
		Usage: &dto.Usage{
			InputTokens:            10,
			OutputTokens:           8,
			TotalTokens:            18,
			CompletionTokenDetails: dto.OutputTokenDetails{ReasoningTokens: 5},
		},
	}

	converted, _, err := ResponsesResponseToChatCompletionsResponse(resp, "resp_1")

	require.NoError(t, err)
	require.Len(t, converted.Choices, 1)
	require.Equal(t, "答案", converted.Choices[0].Message.StringContent())
	require.Equal(t, "先分析问题。", converted.Choices[0].Message.ReasoningContent)
	require.Equal(t, 5, converted.Usage.CompletionTokenDetails.ReasoningTokens)
}

func TestResponsesResponseToChatCompletionsKeepsTextAndToolCalls(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		Model: "gpt-5",
		Output: []dto.ResponsesOutput{
			{
				Type:    "message",
				Role:    "assistant",
				Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: "我先读一下这个文件"}},
			},
			{
				Type:      "function_call",
				CallId:    "call_1",
				Name:      "read",
				Arguments: json.RawMessage(`{"path":"/tmp/a.txt"}`),
			},
		},
	}

	converted, _, err := ResponsesResponseToChatCompletionsResponse(resp, "resp_1")

	require.NoError(t, err)
	require.Equal(t, "我先读一下这个文件", converted.Choices[0].Message.StringContent())
	require.Len(t, converted.Choices[0].Message.ParseToolCalls(), 1)
	require.Equal(t, "read", converted.Choices[0].Message.ParseToolCalls()[0].Function.Name)
	require.Equal(t, "tool_calls", converted.Choices[0].FinishReason)
}

func TestResponsesResponseToChatCompletionsMapsTruncationToLength(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		Model:             "gpt-5",
		IncompleteDetails: &dto.IncompleteDetails{Reason: "max_output_tokens"},
		Output: []dto.ResponsesOutput{
			{
				Type:    "message",
				Role:    "assistant",
				Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: "截断的答"}},
			},
		},
	}

	converted, _, err := ResponsesResponseToChatCompletionsResponse(resp, "resp_1")

	require.NoError(t, err)
	require.Equal(t, "length", converted.Choices[0].FinishReason)
}
