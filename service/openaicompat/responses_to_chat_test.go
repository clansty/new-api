package openaicompat

import (
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
