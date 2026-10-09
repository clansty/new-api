package openaicompat

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

func ResponsesResponseToChatCompletionsResponse(resp *dto.OpenAIResponsesResponse, id string) (*dto.OpenAITextResponse, *dto.Usage, error) {
	if resp == nil {
		return nil, nil, errors.New("response is nil")
	}

	text := ExtractOutputTextFromResponses(resp)
	reasoning := ExtractReasoningTextFromResponses(resp)

	usage := &dto.Usage{}
	if resp.Usage != nil {
		if resp.Usage.InputTokens != 0 {
			usage.PromptTokens = resp.Usage.InputTokens
			usage.InputTokens = resp.Usage.InputTokens
		}
		if resp.Usage.OutputTokens != 0 {
			usage.CompletionTokens = resp.Usage.OutputTokens
			usage.OutputTokens = resp.Usage.OutputTokens
		}
		if resp.Usage.TotalTokens != 0 {
			usage.TotalTokens = resp.Usage.TotalTokens
		} else {
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		}
		if resp.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = resp.Usage.InputTokensDetails.CachedTokens
			usage.PromptTokensDetails.ImageTokens = resp.Usage.InputTokensDetails.ImageTokens
			usage.PromptTokensDetails.AudioTokens = resp.Usage.InputTokensDetails.AudioTokens
		}
		if resp.Usage.CompletionTokenDetails.ReasoningTokens != 0 {
			usage.CompletionTokenDetails.ReasoningTokens = resp.Usage.CompletionTokenDetails.ReasoningTokens
		}
	}

	created := resp.CreatedAt

	// 正文与工具调用并存是常态（模型先说一句再调工具）。旧实现在 text 非空时整段跳过工具提取，
	// 于这类响应里 tool_calls 被静默吞掉，客户端 agent 循环直接卡住。
	var toolCalls []dto.ToolCallResponse
	for _, out := range resp.Output {
		if out.Type != "function_call" {
			continue
		}
		name := strings.TrimSpace(out.Name)
		if name == "" {
			continue
		}
		callId := strings.TrimSpace(out.CallId)
		if callId == "" {
			callId = strings.TrimSpace(out.ID)
		}
		toolCalls = append(toolCalls, dto.ToolCallResponse{
			ID:   callId,
			Type: "function",
			Function: dto.FunctionResponse{
				Name:      name,
				Arguments: out.ArgumentsString(),
			},
		})
	}

	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	} else if ResponsesIncompleteByLength(resp) {
		// 上游因 max_output_tokens 截断时报 stop，会让客户端以为回答完整、无法触发续写。
		finishReason = "length"
	}

	msg := dto.Message{
		Role:    "assistant",
		Content: text,
	}
	msg.ReasoningContent = reasoning
	if len(toolCalls) > 0 {
		msg.SetToolCalls(toolCalls)
	}

	out := &dto.OpenAITextResponse{
		Id:      id,
		Object:  "chat.completion",
		Created: created,
		Model:   resp.Model,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index:        0,
				Message:      msg,
				FinishReason: finishReason,
			},
		},
		Usage: *usage,
	}

	return out, usage, nil
}

// responsesIncompleteByLength 判断 Responses 响应是否因为输出长度上限被截断。
func ResponsesIncompleteByLength(resp *dto.OpenAIResponsesResponse) bool {
	if resp == nil || resp.IncompleteDetails == nil {
		return false
	}
	switch resp.IncompleteDetails.Reason {
	case "max_output_tokens", "max_tokens":
		return true
	}
	return false
}

func ExtractReasoningTextFromResponses(resp *dto.OpenAIResponsesResponse) string {
	if resp == nil || len(resp.Output) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, out := range resp.Output {
		if out.Type != "reasoning" {
			continue
		}
		before := sb.Len()
		for _, part := range out.Summary {
			if part.Text != "" {
				sb.WriteString(part.Text)
			}
		}
		if sb.Len() == before {
			for _, part := range out.Content {
				if part.Text != "" {
					sb.WriteString(part.Text)
				}
			}
		}
	}
	return sb.String()
}

func ExtractOutputTextFromResponses(resp *dto.OpenAIResponsesResponse) string {
	if resp == nil || len(resp.Output) == 0 {
		return ""
	}

	var sb strings.Builder

	// Prefer assistant message outputs.
	for _, out := range resp.Output {
		if out.Type != "message" {
			continue
		}
		if out.Role != "" && out.Role != "assistant" {
			continue
		}
		for _, c := range out.Content {
			if c.Type == "output_text" && c.Text != "" {
				sb.WriteString(c.Text)
			}
		}
	}
	if sb.Len() > 0 {
		return sb.String()
	}
	for _, out := range resp.Output {
		for _, c := range out.Content {
			if c.Text != "" {
				sb.WriteString(c.Text)
			}
		}
	}
	return sb.String()
}
