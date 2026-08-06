package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionsRequestToResponsesRequestPreservesAssistantReasoningHistory(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model: "deepseek-v4-flash",
		Messages: []dto.Message{
			{Role: "user", Content: "问题"},
			{Role: "assistant", Content: "答案", ReasoningContent: "先分析问题。"},
		},
	}

	converted, err := ChatCompletionsRequestToResponsesRequest(req)

	require.NoError(t, err)
	var items []map[string]any
	require.NoError(t, common.Unmarshal(converted.Input, &items))
	require.Len(t, items, 3)
	require.Equal(t, "reasoning", items[1]["type"])
	require.Equal(t, "assistant", items[2]["role"])
	require.Equal(t, "先分析问题。", items[1]["summary"].([]any)[0].(map[string]any)["text"])
}
