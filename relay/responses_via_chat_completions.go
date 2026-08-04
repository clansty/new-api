package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func responsesViaChatCompletions(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, request *dto.OpenAIResponsesRequest) (*dto.Usage, *types.NewAPIError) {
	chatRequest, err := convertResponsesRequestToChatCompletions(request, info)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	defer func() {
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
	}()

	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RequestURLPath = "/v1/chat/completions"
	info.AppendRequestConversion(types.RelayFormatOpenAI)

	convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, chatRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	requestData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	requestData, err = relaycommon.RemoveDisabledFields(requestData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	if len(info.ParamOverride) > 0 {
		requestData, err = relaycommon.ApplyParamOverrideWithRelayInfo(requestData, info)
		if err != nil {
			return nil, newAPIErrorFromParamOverride(err)
		}
	}

	response, err := adaptor.DoRequest(c, info, bytes.NewBuffer(requestData))
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	if response == nil {
		return nil, types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	httpResponse, ok := response.(*http.Response)
	if !ok {
		return nil, types.NewError(fmt.Errorf("unexpected chat completions response type %T", response), types.ErrorCodeBadResponse)
	}
	if httpResponse.StatusCode != http.StatusOK {
		newAPIError := service.RelayErrorHandler(c.Request.Context(), httpResponse, false)
		service.ResetStatusCode(newAPIError, c.GetString("status_code_mapping"))
		return nil, newAPIError
	}

	info.IsStream = info.IsStream || strings.HasPrefix(httpResponse.Header.Get("Content-Type"), "text/event-stream")
	if info.IsStream {
		return chatCompletionsStreamToResponses(c, info, httpResponse)
	}
	return chatCompletionsResponseToResponses(c, httpResponse)
}

func convertResponsesRequestToChatCompletions(request *dto.OpenAIResponsesRequest, info *relaycommon.RelayInfo) (*dto.GeneralOpenAIRequest, error) {
	if request == nil {
		return nil, fmt.Errorf("responses request is nil")
	}
	if err := validateResponsesRequestForChatCompletions(request); err != nil {
		return nil, err
	}

	requestForMessages := *request
	requestForMessages.Text = nil
	claudeRequest, _, _, err := claude.ConvertResponsesRequestToClaude(&requestForMessages)
	if err != nil {
		return nil, err
	}
	chatRequest, err := service.ClaudeToOpenAIRequest(*claudeRequest, info)
	if err != nil {
		return nil, err
	}

	chatRequest.Metadata = request.Metadata
	chatRequest.Store = request.Store
	chatRequest.User = request.User
	chatRequest.PromptCacheRetention = request.PromptCacheRetention
	chatRequest.SafetyIdentifier = request.SafetyIdentifier
	chatRequest.EnableThinking = request.EnableThinking
	chatRequest.TopLogProbs = request.TopLogProbs
	if request.TopLogProbs != nil {
		chatRequest.LogProbs = common.GetPointer(true)
	}
	if request.Reasoning != nil {
		chatRequest.ReasoningEffort = request.Reasoning.Effort
	}
	if request.StreamOptions != nil {
		chatRequest.StreamOptions = request.StreamOptions
	}
	if request.ParallelToolCalls != nil {
		var parallelToolCalls bool
		if err := common.Unmarshal(request.ParallelToolCalls, &parallelToolCalls); err != nil {
			return nil, fmt.Errorf("invalid parallel_tool_calls: %w", err)
		}
		chatRequest.ParallelTooCalls = common.GetPointer(parallelToolCalls)
	}
	if err := applyResponsesToolChoiceToChatCompletions(request.ToolChoice, chatRequest); err != nil {
		return nil, err
	}
	if request.ServiceTier != "" {
		serviceTier, err := common.Marshal(request.ServiceTier)
		if err != nil {
			return nil, fmt.Errorf("marshal service_tier: %w", err)
		}
		chatRequest.ServiceTier = serviceTier
	}
	if len(request.PromptCacheKey) > 0 && !isResponsesChatJSONNull(request.PromptCacheKey) {
		var promptCacheKey string
		if err := common.Unmarshal(request.PromptCacheKey, &promptCacheKey); err != nil {
			return nil, fmt.Errorf("invalid prompt_cache_key: %w", err)
		}
		chatRequest.PromptCacheKey = promptCacheKey
	}
	if err := applyResponsesTextToChatCompletions(request.Text, chatRequest); err != nil {
		return nil, err
	}
	return chatRequest, nil
}

func applyResponsesToolChoiceToChatCompletions(raw json.RawMessage, chatRequest *dto.GeneralOpenAIRequest) error {
	if len(raw) == 0 || isResponsesChatJSONNull(raw) {
		return nil
	}
	var toolChoice any
	if err := common.Unmarshal(raw, &toolChoice); err != nil {
		return fmt.Errorf("invalid tool_choice: %w", err)
	}
	if choice, ok := toolChoice.(string); ok {
		chatRequest.ToolChoice = choice
		return nil
	}
	choice, ok := toolChoice.(map[string]any)
	if !ok {
		return fmt.Errorf("tool_choice must be a string or object")
	}
	choiceType, _ := choice["type"].(string)
	if choiceType != "function" && choiceType != "custom" {
		return fmt.Errorf("tool_choice type %q is not supported when converting Responses to Chat Completions", choiceType)
	}
	name, _ := choice["name"].(string)
	if name == "" {
		return fmt.Errorf("tool_choice.name is required")
	}
	if choiceType == "function" {
		chatRequest.ToolChoice = map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": name,
			},
		}
		return nil
	}
	chatRequest.ToolChoice = map[string]any{"type": "custom", "custom": map[string]any{"name": name}}
	return nil
}

func validateResponsesRequestForChatCompletions(request *dto.OpenAIResponsesRequest) error {
	unsupported := []struct {
		name  string
		value json.RawMessage
	}{
		{name: "conversation", value: request.Conversation},
		{name: "context_management", value: request.ContextManagement},
		{name: "truncation", value: request.Truncation},
		{name: "prompt", value: request.Prompt},
		{name: "preset", value: request.Preset},
	}
	for _, field := range unsupported {
		if len(field.value) > 0 && !isResponsesChatJSONNull(field.value) {
			return fmt.Errorf("%s is not supported when converting Responses to Chat Completions", field.name)
		}
	}
	if request.PreviousResponseID != "" {
		return fmt.Errorf("previous_response_id is not supported when converting Responses to Chat Completions")
	}
	if request.MaxToolCalls != nil {
		return fmt.Errorf("max_tool_calls is not supported when converting Responses to Chat Completions")
	}
	return nil
}

func isResponsesChatJSONNull(value json.RawMessage) bool {
	return strings.TrimSpace(string(value)) == "null"
}

func applyResponsesTextToChatCompletions(raw json.RawMessage, chatRequest *dto.GeneralOpenAIRequest) error {
	if len(raw) == 0 || isResponsesChatJSONNull(raw) {
		return nil
	}
	var text struct {
		Format    json.RawMessage `json:"format"`
		Verbosity json.RawMessage `json:"verbosity"`
	}
	if err := common.Unmarshal(raw, &text); err != nil {
		return fmt.Errorf("invalid text: %w", err)
	}
	chatRequest.Verbosity = text.Verbosity
	if len(text.Format) == 0 || isResponsesChatJSONNull(text.Format) {
		return nil
	}
	var format map[string]any
	if err := common.Unmarshal(text.Format, &format); err != nil {
		return fmt.Errorf("invalid text.format: %w", err)
	}
	formatType, _ := format["type"].(string)
	if formatType == "" {
		return fmt.Errorf("text.format.type is required")
	}
	delete(format, "type")
	responseFormat := &dto.ResponseFormat{Type: formatType}
	if formatType == "json_schema" {
		formatRaw, err := common.Marshal(format)
		if err != nil {
			return fmt.Errorf("marshal text.format schema: %w", err)
		}
		responseFormat.JsonSchema = formatRaw
	}
	chatRequest.ResponseFormat = responseFormat
	return nil
}

func chatCompletionsResponseToResponses(c *gin.Context, response *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(response)
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	var chatResponse dto.OpenAITextResponse
	if err := common.Unmarshal(responseBody, &chatResponse); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if openAIError := chatResponse.GetOpenAIError(); openAIError != nil && openAIError.Type != "" {
		return nil, types.WithOpenAIError(*openAIError, response.StatusCode)
	}

	responsesResponse, usage := chatCompletionsResponseAsResponses(&chatResponse)
	encodedResponse, err := common.Marshal(responsesResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, response, encodedResponse)
	return usage, nil
}

func chatCompletionsResponseAsResponses(response *dto.OpenAITextResponse) (*dto.OpenAIResponsesResponse, *dto.Usage) {
	state := newChatCompletionsResponsesState(response.Model)
	state.Handle(chatCompletionsResponseAsStream(response))
	finalEvents := state.FinalEvents()
	if len(finalEvents) == 0 || finalEvents[0].Response == nil {
		return &dto.OpenAIResponsesResponse{Object: "response", Model: response.Model}, &dto.Usage{}
	}
	return finalEvents[0].Response, finalEvents[0].Response.Usage
}

func chatCompletionsResponseAsStream(response *dto.OpenAITextResponse) *dto.ChatCompletionsStreamResponse {
	chunk := &dto.ChatCompletionsStreamResponse{
		Id:      response.Id,
		Created: responseCreatedAt(response.Created),
		Model:   response.Model,
		Usage:   &response.Usage,
	}
	for _, choice := range response.Choices {
		finishReason := choice.FinishReason
		delta := dto.ChatCompletionsStreamResponseChoiceDelta{Role: choice.Message.Role}
		if content := choice.Message.StringContent(); content != "" {
			delta.Content = common.GetPointer(content)
		}
		toolCalls := choice.Message.ParseToolCalls()
		for index, toolCall := range toolCalls {
			delta.ToolCalls = append(delta.ToolCalls, dto.ToolCallResponse{
				Index: common.GetPointer(index),
				ID:    toolCall.ID,
				Type:  toolCall.Type,
				Function: dto.FunctionResponse{
					Name:      toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
				},
			})
		}
		chunk.Choices = append(chunk.Choices, dto.ChatCompletionsStreamResponseChoice{
			Index:        choice.Index,
			Delta:        delta,
			FinishReason: &finishReason,
		})
	}
	return chunk
}

func responseCreatedAt(value any) int64 {
	switch createdAt := value.(type) {
	case int:
		return int64(createdAt)
	case int64:
		return createdAt
	case float64:
		return int64(createdAt)
	case json.Number:
		parsed, _ := createdAt.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(createdAt, 10, 64)
		return parsed
	default:
		return 0
	}
}

type chatCompletionsToolState struct {
	blockIndex int
	started    bool
	closed     bool
	id         string
	name       string
	arguments  strings.Builder
}

type chatCompletionsResponsesState struct {
	responsesState  *claude.ClaudeResponsesStreamState
	started         bool
	finished        bool
	nextBlockIndex  int
	textBlockIndex  int
	textStarted     bool
	thinkingIndex   int
	thinkingStarted bool
	tools           map[int]*chatCompletionsToolState
	toolOrder       []int
}

func newChatCompletionsResponsesState(model string) *chatCompletionsResponsesState {
	return &chatCompletionsResponsesState{
		responsesState: claude.NewClaudeResponsesStreamState(model),
		textBlockIndex: -1,
		thinkingIndex:  -1,
		tools:          make(map[int]*chatCompletionsToolState),
	}
}

func (s *chatCompletionsResponsesState) Handle(chunk *dto.ChatCompletionsStreamResponse) []dto.ResponsesStreamResponse {
	if chunk == nil || s.finished {
		return nil
	}
	events := make([]dto.ResponsesStreamResponse, 0, 8)
	if !s.started {
		events = append(events, s.start(chunk)...)
	}
	for _, choice := range chunk.Choices {
		events = append(events, s.handleChoice(choice)...)
	}
	if chunk.Usage != nil {
		events = append(events, s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{
			Type:  "message_delta",
			Usage: chatUsageToClaudeUsage(chunk.Usage),
		})...)
	}
	return events
}

func (s *chatCompletionsResponsesState) FinalEvents() []dto.ResponsesStreamResponse {
	if s.finished {
		return nil
	}
	s.finished = true
	events := s.closeOpenBlocks()
	events = append(events, s.responsesState.FinalEvents()...)
	return events
}

func (s *chatCompletionsResponsesState) start(chunk *dto.ChatCompletionsStreamResponse) []dto.ResponsesStreamResponse {
	s.started = true
	return s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{
		Type: "message_start",
		Message: &dto.ClaudeMediaMessage{
			Id:    chunk.Id,
			Model: chunk.Model,
			Usage: chatUsageToClaudeUsage(chunk.Usage),
		},
	})
}

func (s *chatCompletionsResponsesState) handleChoice(choice dto.ChatCompletionsStreamResponseChoice) []dto.ResponsesStreamResponse {
	events := make([]dto.ResponsesStreamResponse, 0, 8)
	if content := choice.Delta.GetContentString(); content != "" {
		events = append(events, s.appendText(content)...)
	}
	if reasoning := choice.Delta.GetReasoningContent(); reasoning != "" {
		events = append(events, s.appendThinking(reasoning, choice.Delta.GetReasoningOpaque())...)
	}
	for index, toolCall := range choice.Delta.ToolCalls {
		toolIndex := index
		if toolCall.Index != nil {
			toolIndex = *toolCall.Index
		}
		events = append(events, s.appendToolCall(toolIndex, toolCall)...)
	}
	if choice.FinishReason != nil && *choice.FinishReason != "" {
		events = append(events, s.closeOpenBlocks()...)
		events = append(events, s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{
			Type:  "message_delta",
			Delta: &dto.ClaudeMediaMessage{StopReason: common.GetPointer(chatFinishReasonToClaude(*choice.FinishReason))},
		})...)
	}
	return events
}

func (s *chatCompletionsResponsesState) appendText(content string) []dto.ResponsesStreamResponse {
	events := make([]dto.ResponsesStreamResponse, 0, 3)
	if !s.textStarted {
		s.textStarted = true
		s.textBlockIndex = s.nextBlockIndex
		s.nextBlockIndex++
		events = append(events, s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{
			Type:         "content_block_start",
			Index:        common.GetPointer(s.textBlockIndex),
			ContentBlock: &dto.ClaudeMediaMessage{Type: "text", Text: common.GetPointer("")},
		})...)
	}
	events = append(events, s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: common.GetPointer(s.textBlockIndex),
		Delta: &dto.ClaudeMediaMessage{Type: "text_delta", Text: common.GetPointer(content)},
	})...)
	return events
}

func (s *chatCompletionsResponsesState) appendThinking(reasoning string, opaque string) []dto.ResponsesStreamResponse {
	events := make([]dto.ResponsesStreamResponse, 0, 4)
	if !s.thinkingStarted {
		s.thinkingStarted = true
		s.thinkingIndex = s.nextBlockIndex
		s.nextBlockIndex++
		events = append(events, s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{
			Type:         "content_block_start",
			Index:        common.GetPointer(s.thinkingIndex),
			ContentBlock: &dto.ClaudeMediaMessage{Type: "thinking", Thinking: common.GetPointer(""), Signature: common.GetPointer("")},
		})...)
	}
	events = append(events, s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: common.GetPointer(s.thinkingIndex),
		Delta: &dto.ClaudeMediaMessage{Type: "thinking_delta", Thinking: common.GetPointer(reasoning)},
	})...)
	if opaque != "" {
		events = append(events, s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{
			Type:  "content_block_delta",
			Index: common.GetPointer(s.thinkingIndex),
			Delta: &dto.ClaudeMediaMessage{Type: "signature_delta", Signature: common.GetPointer(opaque)},
		})...)
	}
	return events
}

func (s *chatCompletionsResponsesState) appendToolCall(index int, toolCall dto.ToolCallResponse) []dto.ResponsesStreamResponse {
	tool, found := s.tools[index]
	if !found {
		tool = &chatCompletionsToolState{blockIndex: s.nextBlockIndex}
		s.nextBlockIndex++
		s.tools[index] = tool
		s.toolOrder = append(s.toolOrder, index)
	}
	if toolCall.ID != "" {
		tool.id = toolCall.ID
	}
	if toolCall.Function.Name != "" {
		tool.name = toolCall.Function.Name
	}
	if toolCall.Function.Arguments != "" {
		tool.arguments.WriteString(toolCall.Function.Arguments)
	}
	if !tool.started && (tool.id != "" || tool.name != "") {
		tool.started = true
		events := s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{
			Type:         "content_block_start",
			Index:        common.GetPointer(tool.blockIndex),
			ContentBlock: &dto.ClaudeMediaMessage{Type: "tool_use", Id: tool.id, Name: tool.name},
		})
		if arguments := tool.arguments.String(); arguments != "" {
			events = append(events, s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{
				Type:  "content_block_delta",
				Index: common.GetPointer(tool.blockIndex),
				Delta: &dto.ClaudeMediaMessage{Type: "input_json_delta", PartialJson: common.GetPointer(arguments)},
			})...)
		}
		return events
	}
	if !tool.started || toolCall.Function.Arguments == "" {
		return nil
	}
	return s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: common.GetPointer(tool.blockIndex),
		Delta: &dto.ClaudeMediaMessage{Type: "input_json_delta", PartialJson: common.GetPointer(toolCall.Function.Arguments)},
	})
}

func (s *chatCompletionsResponsesState) closeOpenBlocks() []dto.ResponsesStreamResponse {
	events := make([]dto.ResponsesStreamResponse, 0, len(s.tools)+2)
	if s.textStarted {
		events = append(events, s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{Type: "content_block_stop", Index: common.GetPointer(s.textBlockIndex)})...)
		s.textStarted = false
	}
	if s.thinkingStarted {
		events = append(events, s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{Type: "content_block_stop", Index: common.GetPointer(s.thinkingIndex)})...)
		s.thinkingStarted = false
	}
	for _, toolIndex := range s.toolOrder {
		tool := s.tools[toolIndex]
		if tool.closed {
			continue
		}
		if !tool.started {
			tool.started = true
			events = append(events, s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{
				Type:         "content_block_start",
				Index:        common.GetPointer(tool.blockIndex),
				ContentBlock: &dto.ClaudeMediaMessage{Type: "tool_use", Id: tool.id, Name: tool.name},
			})...)
			if arguments := tool.arguments.String(); arguments != "" {
				events = append(events, s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{
					Type:  "content_block_delta",
					Index: common.GetPointer(tool.blockIndex),
					Delta: &dto.ClaudeMediaMessage{Type: "input_json_delta", PartialJson: common.GetPointer(arguments)},
				})...)
			}
		}
		events = append(events, s.responsesState.HandleClaudeChunk(&dto.ClaudeResponse{Type: "content_block_stop", Index: common.GetPointer(tool.blockIndex)})...)
		tool.closed = true
	}
	return events
}

func chatUsageToClaudeUsage(usage *dto.Usage) *dto.ClaudeUsage {
	if usage == nil {
		return nil
	}
	return &dto.ClaudeUsage{
		InputTokens:              usage.PromptTokens,
		OutputTokens:             usage.CompletionTokens,
		CacheReadInputTokens:     usage.PromptTokensDetails.CachedTokens,
		CacheCreationInputTokens: usage.PromptTokensDetails.CachedCreationTokens,
	}
}

func chatFinishReasonToClaude(reason string) string {
	switch reason {
	case "length":
		return "max_tokens"
	case "tool_calls", "function_call":
		return "tool_use"
	default:
		return "end_turn"
	}
}

func chatCompletionsStreamToResponses(c *gin.Context, info *relaycommon.RelayInfo, response *http.Response) (*dto.Usage, *types.NewAPIError) {
	state := newChatCompletionsResponsesState(info.UpstreamModelName)
	var streamError *types.NewAPIError
	helper.StreamScannerHandler(c, response, info, func(data string, streamResult *helper.StreamResult) {
		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			streamError = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			streamResult.Stop(streamError)
			return
		}
		for _, event := range state.Handle(&chunk) {
			if err := writeResponsesStreamEvent(c, event); err != nil {
				streamError = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
				streamResult.Stop(streamError)
				return
			}
		}
	})
	if streamError != nil {
		return nil, streamError
	}
	finalEvents := state.FinalEvents()
	var usage *dto.Usage
	for _, event := range finalEvents {
		if event.Response != nil {
			usage = event.Response.Usage
		}
		if err := writeResponsesStreamEvent(c, event); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
	}
	if usage == nil {
		usage = &dto.Usage{}
	}
	if usage.TotalTokens == 0 {
		for _, event := range finalEvents {
			if event.Response != nil {
				usage = service.ResponseText2Usage(c, service.ExtractOutputTextFromResponses(event.Response), info.UpstreamModelName, info.GetEstimatePromptTokens())
				break
			}
		}
	}
	return usage, nil
}

func writeResponsesStreamEvent(c *gin.Context, event dto.ResponsesStreamResponse) error {
	data, err := common.Marshal(event)
	if err != nil {
		return err
	}
	helper.ResponseChunkData(c, event, string(data))
	return nil
}
