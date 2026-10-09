package service

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service/openaicompat"
)

func ChatCompletionsRequestToResponsesRequest(req *dto.GeneralOpenAIRequest) (*dto.OpenAIResponsesRequest, error) {
	return openaicompat.ChatCompletionsRequestToResponsesRequest(req)
}

func ClaudeRequestToResponsesRequest(req *dto.ClaudeRequest) (*dto.OpenAIResponsesRequest, error) {
	return openaicompat.ClaudeRequestToResponsesRequest(req)
}

func ResponsesResponseToChatCompletionsResponse(resp *dto.OpenAIResponsesResponse, id string) (*dto.OpenAITextResponse, *dto.Usage, error) {
	return openaicompat.ResponsesResponseToChatCompletionsResponse(resp, id)
}

func ExtractOutputTextFromResponses(resp *dto.OpenAIResponsesResponse) string {
	return openaicompat.ExtractOutputTextFromResponses(resp)
}

// ResponsesIncompleteByLength 判断 Responses 响应是否因为输出长度上限被截断。
func ResponsesIncompleteByLength(resp *dto.OpenAIResponsesResponse) bool {
	return openaicompat.ResponsesIncompleteByLength(resp)
}
