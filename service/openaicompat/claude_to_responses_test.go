package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func claudeToolResultRequestOutput(t *testing.T, block dto.ClaudeMediaMessage) any {
	t.Helper()
	req, err := ClaudeRequestToResponsesRequest(&dto.ClaudeRequest{
		Model:    "gpt-5",
		Messages: []dto.ClaudeMessage{{Role: "user", Content: []dto.ClaudeMediaMessage{block}}},
	})
	require.NoError(t, err)

	var input []map[string]any
	require.NoError(t, common.Unmarshal(req.Input, &input))
	require.Len(t, input, 1)
	return input[0]["output"]
}

func TestConvertClaudeToolResultOutputKeepsPlainTextAsString(t *testing.T) {
	output := claudeToolResultRequestOutput(t, dto.ClaudeMediaMessage{
		Type:      "tool_result",
		ToolUseId: "call_1",
		Content:   "42",
	})

	require.Equal(t, "42", output)
}

func TestConvertClaudeToolResultOutputMarksErrors(t *testing.T) {
	output := claudeToolResultRequestOutput(t, dto.ClaudeMediaMessage{
		Type:      "tool_result",
		ToolUseId: "call_1",
		Content:   "file not found",
		IsError:   true,
	})

	require.Equal(t, "[tool_result_error]\nfile not found", output)
}

func TestConvertClaudeToolResultOutputKeepsEmbeddedImageStructured(t *testing.T) {
	output := claudeToolResultRequestOutput(t, dto.ClaudeMediaMessage{
		Type:      "tool_result",
		ToolUseId: "call_1",
		Content: []dto.ClaudeMediaMessage{
			{Type: "text", Text: common.GetPointer("截图如下")},
			{Type: "image", Source: &dto.ClaudeMessageSource{
				Type:      "base64",
				MediaType: "image/png",
				Data:      "iVBORw0KGgo=",
			}},
		},
	})

	// 图片必须作为结构化内容块透传，模型才看得到；旧实现会把整段 JSON 塞成一个字符串。
	parts, ok := output.([]any)
	require.True(t, ok, "expected structured content parts, got %T", output)
	require.Len(t, parts, 2)
	first := parts[0].(map[string]any)
	require.Equal(t, "input_text", first["type"])
	require.Equal(t, "截图如下", first["text"])
	second := parts[1].(map[string]any)
	require.Equal(t, "input_image", second["type"])
	require.Equal(t, "data:image/png;base64,iVBORw0KGgo=", second["image_url"])
}
