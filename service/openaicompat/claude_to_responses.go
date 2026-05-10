package openaicompat

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

type claudeOutputConfigForResponses struct {
	Effort string         `json:"effort,omitempty"`
	Format map[string]any `json:"format,omitempty"`
}

type claudeOutputFormatForResponses struct {
	Type string `json:"type,omitempty"`
}

type claudeMetadataForResponses struct {
	UserID string `json:"user_id,omitempty"`
}

func ClaudeRequestToResponsesRequest(req *dto.ClaudeRequest) (*dto.OpenAIResponsesRequest, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if req.Model == "" {
		return nil, errors.New("model is required")
	}

	instructionsRaw, err := convertClaudeSystemToResponsesInstructions(req.System)
	if err != nil {
		return nil, err
	}

	input, err := convertClaudeMessagesToResponsesInput(req.Messages)
	if err != nil {
		return nil, err
	}
	inputRaw, err := common.Marshal(input)
	if err != nil {
		return nil, err
	}

	toolsRaw, err := convertClaudeToolsToResponsesTools(req.Tools)
	if err != nil {
		return nil, err
	}

	toolChoiceRaw, parallelToolCallsRaw, err := convertClaudeToolChoiceToResponses(req.ToolChoice)
	if err != nil {
		return nil, err
	}

	textRaw, err := convertClaudeOutputToResponsesText(req.OutputConfig, req.OutputFormat)
	if err != nil {
		return nil, err
	}

	userRaw, err := convertClaudeMetadataToResponsesUser(req.Metadata)
	if err != nil {
		return nil, err
	}

	maxOutputTokens := lo.FromPtrOr(req.MaxTokens, uint(0))
	if maxOutputTokens > 0 && maxOutputTokens < 16 {
		maxOutputTokens = 16
	}

	out := &dto.OpenAIResponsesRequest{
		Model:             req.Model,
		Input:             inputRaw,
		Instructions:      instructionsRaw,
		Stream:            req.Stream,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		Tools:             toolsRaw,
		ToolChoice:        toolChoiceRaw,
		ParallelToolCalls: parallelToolCallsRaw,
		Text:              textRaw,
		Metadata:          req.Metadata,
		User:              userRaw,
		ServiceTier:       req.ServiceTier,
	}
	if len(req.StopSequences) == 1 {
		out.Stop = req.StopSequences[0]
	} else if len(req.StopSequences) > 1 {
		out.Stop = req.StopSequences
	}
	if req.MaxTokens != nil {
		out.MaxOutputTokens = common.GetPointer(maxOutputTokens)
	}
	if effort := resolveClaudeReasoningEffort(req); effort != "" {
		out.Reasoning = &dto.Reasoning{
			Effort:  mapClaudeReasoningEffortToResponses(effort),
			Summary: "detailed",
		}
	}

	return out, nil
}

func convertClaudeSystemToResponsesInstructions(system any) ([]byte, error) {
	if system == nil {
		return nil, nil
	}
	if s, ok := system.(string); ok {
		if strings.TrimSpace(s) == "" {
			return nil, nil
		}
		return common.Marshal(s)
	}

	systems, err := common.Any2Type[[]dto.ClaudeMediaMessage](system)
	if err != nil {
		return nil, err
	}
	parts := make([]string, 0, len(systems))
	for _, block := range systems {
		if block.Type != "" && block.Type != dto.ContentTypeText {
			continue
		}
		if text := strings.TrimSpace(block.GetText()); text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return nil, nil
	}
	return common.Marshal(strings.Join(parts, "\n\n"))
}

func convertClaudeMessagesToResponsesInput(messages []dto.ClaudeMessage) ([]map[string]any, error) {
	input := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		if message.Role == "user" {
			if err := appendClaudeUserMessageAsResponsesInput(&input, message); err != nil {
				return nil, err
			}
			continue
		}
		if err := appendClaudeAssistantMessageAsResponsesInput(&input, message); err != nil {
			return nil, err
		}
	}
	return input, nil
}

func appendClaudeUserMessageAsResponsesInput(input *[]map[string]any, message dto.ClaudeMessage) error {
	if message.IsStringContent() {
		*input = append(*input, map[string]any{
			"role":    "user",
			"content": message.GetStringContent(),
		})
		return nil
	}

	blocks, err := message.ParseContent()
	if err != nil {
		return err
	}
	content := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case "tool_result":
			*input = append(*input, map[string]any{
				"type":    "function_call_output",
				"call_id": block.ToolUseId,
				"output":  serializeClaudeToolResultContent(block.Content),
			})
		case dto.ContentTypeText, "input_text":
			content = append(content, map[string]any{"type": "input_text", "text": block.GetText()})
		case "image":
			image, err := convertClaudeImageBlockToResponsesContent(block)
			if err != nil {
				return err
			}
			content = append(content, image)
		case "document":
			return fmt.Errorf("document blocks are not supported in Claude to Responses compatibility mode")
		default:
			content = append(content, map[string]any{"type": block.Type})
		}
	}
	if len(content) > 0 {
		*input = append(*input, map[string]any{"role": "user", "content": content})
	}
	return nil
}

func appendClaudeAssistantMessageAsResponsesInput(input *[]map[string]any, message dto.ClaudeMessage) error {
	if message.IsStringContent() {
		*input = append(*input, map[string]any{
			"role":    "assistant",
			"content": []map[string]any{{"type": "output_text", "text": message.GetStringContent()}},
		})
		return nil
	}

	blocks, err := message.ParseContent()
	if err != nil {
		return err
	}
	flushText := func(textParts []string) {
		if len(textParts) == 0 {
			return
		}
		*input = append(*input, map[string]any{
			"role": "assistant",
			"content": []map[string]any{{
				"type": "output_text",
				"text": strings.Join(textParts, "\n\n"),
			}},
		})
	}

	textParts := make([]string, 0)
	for _, block := range blocks {
		switch block.Type {
		case dto.ContentTypeText, "output_text":
			if text := strings.TrimSpace(block.GetText()); text != "" {
				textParts = append(textParts, text)
			}
		case "tool_use":
			flushText(textParts)
			textParts = textParts[:0]
			arguments, err := common.Marshal(block.Input)
			if err != nil {
				return err
			}
			*input = append(*input, map[string]any{
				"type":      "function_call",
				"id":        "fc_" + block.Id,
				"call_id":   block.Id,
				"name":      block.Name,
				"arguments": string(arguments),
				"status":    "completed",
			})
		}
	}
	flushText(textParts)
	return nil
}

func convertClaudeImageBlockToResponsesContent(block dto.ClaudeMediaMessage) (map[string]any, error) {
	if block.Source == nil {
		return map[string]any{"type": "input_image"}, nil
	}
	if block.Source.Url != "" {
		return map[string]any{"type": "input_image", "image_url": block.Source.Url}, nil
	}
	if block.Source.Data != nil {
		return map[string]any{
			"type":      "input_image",
			"image_url": fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, common.Interface2String(block.Source.Data)),
		}, nil
	}
	return map[string]any{"type": "input_image"}, nil
}

func serializeClaudeToolResultContent(content any) string {
	if content == nil {
		return ""
	}
	if text, ok := content.(string); ok {
		return text
	}
	blocks, err := common.Any2Type[[]dto.ClaudeMediaMessage](content)
	if err == nil {
		texts := make([]string, 0, len(blocks))
		for _, block := range blocks {
			if block.Type != dto.ContentTypeText && block.Type != "" {
				encoded, _ := common.Marshal(content)
				return string(encoded)
			}
			texts = append(texts, block.GetText())
		}
		return strings.Join(texts, "\n\n")
	}
	encoded, _ := common.Marshal(content)
	return string(encoded)
}

func convertClaudeToolsToResponsesTools(tools any) ([]byte, error) {
	if tools == nil {
		return nil, nil
	}
	toolItems, err := normalizeClaudeTools(tools)
	if err != nil {
		return nil, err
	}
	normalTools, webSearchTools := dto.ProcessTools(toolItems)
	if len(webSearchTools) > 0 {
		return nil, fmt.Errorf("web search tools are not supported in Claude to Responses compatibility mode")
	}
	if len(normalTools) == 0 {
		return nil, nil
	}
	responsesTools := make([]map[string]any, 0, len(normalTools))
	for _, tool := range normalTools {
		responsesTools = append(responsesTools, map[string]any{
			"type":        "function",
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.InputSchema,
		})
	}
	return common.Marshal(responsesTools)
}

func normalizeClaudeTools(tools any) ([]any, error) {
	switch typedTools := tools.(type) {
	case []any:
		items := make([]any, 0, len(typedTools))
		for _, tool := range typedTools {
			normalized, err := normalizeClaudeToolItem(tool)
			if err != nil {
				return nil, err
			}
			items = append(items, normalized)
		}
		return items, nil
	case []dto.Tool:
		items := make([]any, 0, len(typedTools))
		for _, tool := range typedTools {
			items = append(items, tool)
		}
		return items, nil
	case []*dto.Tool:
		items := make([]any, 0, len(typedTools))
		for _, tool := range typedTools {
			items = append(items, tool)
		}
		return items, nil
	case []dto.ClaudeWebSearchTool:
		items := make([]any, 0, len(typedTools))
		for _, tool := range typedTools {
			items = append(items, tool)
		}
		return items, nil
	case []*dto.ClaudeWebSearchTool:
		items := make([]any, 0, len(typedTools))
		for _, tool := range typedTools {
			items = append(items, tool)
		}
		return items, nil
	default:
		return common.Any2Type[[]any](tools)
	}
}

func normalizeClaudeToolItem(tool any) (any, error) {
	switch typedTool := tool.(type) {
	case dto.Tool, *dto.Tool, dto.ClaudeWebSearchTool, *dto.ClaudeWebSearchTool:
		return typedTool, nil
	case map[string]any:
		if toolType, _ := typedTool["type"].(string); strings.HasPrefix(toolType, "web_search") {
			webSearchTool, err := common.Any2Type[dto.ClaudeWebSearchTool](typedTool)
			if err != nil {
				return nil, err
			}
			return webSearchTool, nil
		}
		functionTool, err := common.Any2Type[dto.Tool](typedTool)
		if err != nil {
			return nil, err
		}
		return functionTool, nil
	default:
		functionTool, err := common.Any2Type[dto.Tool](typedTool)
		if err != nil {
			return nil, err
		}
		return functionTool, nil
	}
}

func convertClaudeToolChoiceToResponses(toolChoice any) ([]byte, []byte, error) {
	if toolChoice == nil {
		return nil, nil, nil
	}
	choice, err := common.Any2Type[dto.ClaudeToolChoice](toolChoice)
	if err != nil {
		return nil, nil, err
	}
	var mapped any
	switch choice.Type {
	case "auto":
		mapped = "auto"
	case "any":
		mapped = "required"
	case "none":
		mapped = "none"
	case "tool":
		if strings.TrimSpace(choice.Name) != "" {
			mapped = map[string]any{"type": "function", "name": choice.Name}
		}
	}

	var toolChoiceRaw []byte
	if mapped != nil {
		toolChoiceRaw, err = common.Marshal(mapped)
		if err != nil {
			return nil, nil, err
		}
	}

	var parallelToolCallsRaw []byte
	if choice.DisableParallelToolUse {
		parallelToolCallsRaw, err = common.Marshal(false)
		if err != nil {
			return nil, nil, err
		}
	}
	return toolChoiceRaw, parallelToolCallsRaw, nil
}

func convertClaudeOutputToResponsesText(outputConfig []byte, outputFormat []byte) ([]byte, error) {
	formatType, err := getClaudeOutputFormatType(outputConfig, true)
	if err != nil {
		return nil, err
	}
	if formatType == "" {
		formatType, err = getClaudeOutputFormatType(outputFormat, false)
		if err != nil {
			return nil, err
		}
	}
	if formatType != "json_object" {
		return nil, nil
	}
	return common.Marshal(map[string]any{
		"format": map[string]any{"type": "json_object"},
	})
}

func getClaudeOutputFormatType(raw []byte, nested bool) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	if nested {
		var config claudeOutputConfigForResponses
		if err := common.Unmarshal(raw, &config); err != nil {
			return "", err
		}
		formatType, _ := config.Format["type"].(string)
		return formatType, nil
	}
	var format claudeOutputFormatForResponses
	if err := common.Unmarshal(raw, &format); err != nil {
		return "", err
	}
	return format.Type, nil
}

func convertClaudeMetadataToResponsesUser(metadata []byte) ([]byte, error) {
	if len(metadata) == 0 {
		return nil, nil
	}
	var claudeMetadata claudeMetadataForResponses
	if err := common.Unmarshal(metadata, &claudeMetadata); err != nil {
		return nil, err
	}
	if strings.TrimSpace(claudeMetadata.UserID) == "" {
		return nil, nil
	}
	return common.Marshal(claudeMetadata.UserID)
}

func resolveClaudeReasoningEffort(req *dto.ClaudeRequest) string {
	if req == nil {
		return ""
	}
	if req.Thinking != nil && req.Thinking.Type == "disabled" {
		return ""
	}
	if effort := req.GetEfforts(); effort != "" {
		return effort
	}
	if req.Thinking == nil {
		return ""
	}
	switch req.Thinking.Type {
	case "enabled":
		budgetTokens := req.Thinking.GetBudgetTokens()
		if budgetTokens == 0 {
			return ""
		}
		if budgetTokens <= 4096 {
			return "low"
		}
		if budgetTokens <= 16384 {
			return "medium"
		}
		return "high"
	case "adaptive":
		return "high"
	default:
		return ""
	}
}

func mapClaudeReasoningEffortToResponses(effort string) string {
	if effort == "max" {
		return "xhigh"
	}
	return effort
}
