package claude

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// 不走 Chat Completions 中间格式，避免有损翻译丢失 thinking signature 等关键字段。
// 第二个返回值为 OpenAI Responses 里 type:"custom" 工具的名字集合；
// 上游 Anthropic 无 custom tool 概念，所以请求侧降级成 function tool，
// 响应侧需要这个集合把 tool_use 还原成 custom_tool_call 实现透明往返。
func ConvertResponsesRequestToClaude(req *dto.OpenAIResponsesRequest) (*dto.ClaudeRequest, map[string]bool, error) {
	if req == nil {
		return nil, nil, errors.New("request is nil")
	}
	if len(req.PreviousResponseID) > 0 {
		return nil, nil, errors.New("previous_response_id is not supported when converting to Anthropic Messages API; pass the full conversation in input")
	}
	if len(req.Conversation) > 0 && !isJSONNull(req.Conversation) {
		return nil, nil, errors.New("conversation is not supported when converting to Anthropic Messages API")
	}
	if format, present, err := extractTextFormatType(req.Text); err != nil {
		return nil, nil, err
	} else if present && format != "text" {
		return nil, nil, fmt.Errorf("text.format=%q is not supported when converting to Anthropic Messages API", format)
	}

	claude := &dto.ClaudeRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		ServiceTier: req.ServiceTier,
	}
	if req.MaxOutputTokens != nil {
		claude.MaxTokens = req.MaxOutputTokens
	}

	if stops, err := extractStopSequences(req.Stop); err != nil {
		return nil, nil, err
	} else if len(stops) > 0 {
		claude.StopSequences = stops
	}

	system, err := buildSystemFromInstructions(req.Instructions)
	if err != nil {
		return nil, nil, err
	}

	messages, systemPrefixBlocks, err := convertResponsesInputToClaudeMessages(req.Input)
	if err != nil {
		return nil, nil, err
	}
	claude.Messages = messages
	mergedSystem, err := mergeSystem(system, systemPrefixBlocks)
	if err != nil {
		return nil, nil, err
	}
	claude.System = mergedSystem

	tools, customNames, survivingNames, err := convertResponsesToolsToClaudeTools(req.Tools)
	if err != nil {
		return nil, nil, err
	}
	if len(tools) > 0 {
		claude.Tools = tools
	}

	if tc, err := convertResponsesToolChoiceToClaude(req.ToolChoice, survivingNames); err != nil {
		return nil, nil, err
	} else if tc != nil {
		claude.ToolChoice = tc
	}

	if req.Reasoning != nil {
		claude.Thinking = mapResponsesReasoningToClaudeThinking(req.Reasoning)
	}

	if meta, err := convertResponsesMetadataToClaude(req.Metadata); err != nil {
		return nil, nil, err
	} else if meta != nil {
		claude.Metadata = meta
	}

	return claude, customNames, nil
}

func isJSONNull(raw []byte) bool {
	s := strings.TrimSpace(string(raw))
	return s == "" || s == "null"
}

func extractTextFormatType(raw []byte) (formatType string, present bool, err error) {
	if isJSONNull(raw) {
		return "", false, nil
	}
	var text struct {
		Format *struct {
			Type string `json:"type"`
		} `json:"format,omitempty"`
	}
	if err := common.Unmarshal(raw, &text); err != nil {
		return "", false, err
	}
	if text.Format == nil {
		return "", false, nil
	}
	return text.Format.Type, true, nil
}

func extractStopSequences(stop any) ([]string, error) {
	if stop == nil {
		return nil, nil
	}
	switch v := stop.(type) {
	case string:
		if v == "" {
			return nil, nil
		}
		return []string{v}, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("stop sequence item must be string, got %T", item)
			}
			out = append(out, s)
		}
		return out, nil
	}
	return nil, fmt.Errorf("unsupported stop type %T", stop)
}

func buildSystemFromInstructions(raw []byte) (any, error) {
	if isJSONNull(raw) {
		return nil, nil
	}
	var asString string
	if err := common.Unmarshal(raw, &asString); err == nil {
		if asString == "" {
			return nil, nil
		}
		return asString, nil
	}
	var asArray []any
	if err := common.Unmarshal(raw, &asArray); err == nil {
		blocks := make([]dto.ClaudeMediaMessage, 0, len(asArray))
		for _, item := range asArray {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := stringFromInputTextPart(m); ok && text != "" {
				blocks = append(blocks, dto.ClaudeMediaMessage{
					Type: "text",
					Text: common.GetPointer(text),
				})
			}
		}
		if len(blocks) == 0 {
			return nil, nil
		}
		return blocks, nil
	}
	return nil, errors.New("instructions must be a string or an array of input_text parts")
}

func stringFromInputTextPart(m map[string]any) (string, bool) {
	t, _ := m["type"].(string)
	if t != "" && t != "input_text" && t != "text" {
		return "", false
	}
	if text, ok := m["text"].(string); ok {
		return text, true
	}
	return "", false
}

// 把 instructions 来的 system（string 或 []ClaudeMediaMessage）与 input 开头抽出的
// system/developer text blocks 合并。任一为空走快路径；都非空时统一规范成
// []ClaudeMediaMessage，让 Anthropic 上游收到一份扁平 text block 数组。
// 未知类型返回 error 而非 panic：API 网关路径上 panic 会暴露为 500，不如显式 400。
func mergeSystem(instructions any, prefixBlocks []dto.ClaudeMediaMessage) (any, error) {
	if len(prefixBlocks) == 0 {
		return instructions, nil
	}
	if instructions == nil {
		return prefixBlocks, nil
	}
	switch v := instructions.(type) {
	case string:
		head := []dto.ClaudeMediaMessage{{Type: "text", Text: common.GetPointer(v)}}
		return append(head, prefixBlocks...), nil
	case []dto.ClaudeMediaMessage:
		return append(append([]dto.ClaudeMediaMessage{}, v...), prefixBlocks...), nil
	}
	return nil, fmt.Errorf("mergeSystem: unsupported instructions type %T; update buildSystemFromInstructions contract", instructions)
}

// Responses input 数组里可能混合：message / function_call / function_call_output / custom_tool_call / custom_tool_call_output / reasoning。
// 多个相邻同 role 的 item 需要合并到同一个 Claude message 的 content blocks 里，
// 这是 Anthropic 协议的硬要求：thinking → tool_use → text 等都属于同一 assistant turn。
//
// system/developer 在 OpenAI 协议中可穿插于对话中间表达"动态切换规则"，但 Anthropic 没有对应表达。
// 采用三段策略（统称策略 D）：
//   - 出现在任何对话 item 之前（开头）→ 抽到顶层 system 字段（无损）
//   - 穿插在中间 → 用 <openai_developer_instruction> XML 标签包裹成 text block，按下面的规则插入
//   - 末尾（后面没有对话 item）→ append 到末尾 user message，或新建 user message 承载
//
// 插入规则需要同时维护两个 Anthropic 硬约束：
//  1. tool_result 块必须位于 user message 头部（不能被 text 块前置）。
//     若 pending 出现时下一项是 user-with-leading-tool_result，wrapped 进入 deferredWrap，
//     待 tool_result run 结束后再以独立 user message 形式 emit，依靠相邻同 role 合并产生
//     [tool_result*, ..., wrapped] —— 这覆盖并行 tool 调用（多个 function_call_output 连续）。
//  2. assistant 的 tool_use 块必须紧跟 user 的 tool_result（同一 handshake 内不能插任何 text）。
//     若 pending 需要在前 assistant(含 tool_use) 后面注入 user text carrier，直接拒绝为
//     "不可表达"，避免静默产出非法的 Anthropic 请求。
//
// 关键不变量：pending 必须在 **任何非-system item emit 之前** 被处理，否则会出现时序反转或
// assistant turn 合并错误。
//
// 返回值 systemPrefixBlocks 是开头段抽出的 text blocks，由上层与 instructions 合并到 claude.System。
func convertResponsesInputToClaudeMessages(rawInput []byte) ([]dto.ClaudeMessage, []dto.ClaudeMediaMessage, error) {
	if isJSONNull(rawInput) {
		return nil, nil, errors.New("input is required")
	}
	var asString string
	if err := common.Unmarshal(rawInput, &asString); err == nil {
		return []dto.ClaudeMessage{{
			Role:    "user",
			Content: asString,
		}}, nil, nil
	}

	var items []map[string]any
	if err := common.Unmarshal(rawInput, &items); err != nil {
		return nil, nil, fmt.Errorf("input must be a string or an array of items: %w", err)
	}

	messages := make([]dto.ClaudeMessage, 0, len(items))
	var systemPrefixBlocks []dto.ClaudeMediaMessage
	var pendingSystemBlocks []dto.ClaudeMediaMessage
	var deferredWrap []dto.ClaudeMediaMessage
	seenNonSystem := false

	emit := func(role string, blocks []dto.ClaudeMediaMessage) error {
		if n := len(messages); n > 0 && messages[n-1].Role == role {
			if existing, ok := messages[n-1].Content.([]dto.ClaudeMediaMessage); ok {
				if err := assertReasoningOrder(existing, blocks); err != nil {
					return err
				}
				messages[n-1].Content = append(existing, blocks...)
				return nil
			}
		}
		messages = append(messages, dto.ClaudeMessage{Role: role, Content: blocks})
		return nil
	}

	prevIsUser := func() bool {
		return len(messages) > 0 && messages[len(messages)-1].Role == "user"
	}

	lastAssistantHasToolUse := func() bool {
		if len(messages) == 0 {
			return false
		}
		last := messages[len(messages)-1]
		if last.Role != "assistant" {
			return false
		}
		blocks, ok := last.Content.([]dto.ClaudeMediaMessage)
		if !ok {
			return false
		}
		for _, b := range blocks {
			if b.Type == "tool_use" {
				return true
			}
		}
		return false
	}

	appendToLastUserContent := func(blocks []dto.ClaudeMediaMessage) bool {
		if !prevIsUser() {
			return false
		}
		existing, ok := messages[len(messages)-1].Content.([]dto.ClaudeMediaMessage)
		if !ok {
			return false
		}
		messages[len(messages)-1].Content = append(existing, blocks...)
		return true
	}

	flushDeferredWrap := func() error {
		if len(deferredWrap) == 0 {
			return nil
		}
		w := deferredWrap
		deferredWrap = nil
		return emit("user", w)
	}

	emitWithPendingFlush := func(role string, blocks []dto.ClaudeMediaMessage) error {
		isNextToolResultUser := role == "user" && len(blocks) > 0 && blocks[0].Type == "tool_result"

		// deferredWrap 来自更早的 pending dev，被 tool_result run 推迟。
		// 当下一项不再是 leading-tool_result user 时，先 flush（合并到前 user 末尾）。
		if !isNextToolResultUser {
			if err := flushDeferredWrap(); err != nil {
				return err
			}
		}

		if len(pendingSystemBlocks) == 0 {
			return emit(role, blocks)
		}

		// 统一 handshake guard：如果有 pending wrapped 要插入，且前一条 assistant
		// 含未配对的 tool_use，下一项又不是 tool_result user，那么我们插入 wrapped
		// 会破坏 tool_use→tool_result 配对（不管下一项是 user_text 还是 assistant 都一样）。
		if !isNextToolResultUser && lastAssistantHasToolUse() {
			return errors.New("cannot insert system/developer message between assistant tool_use and its matching tool_result; reorder your input")
		}

		wrapped := wrapSystemBlocksAsUserText(pendingSystemBlocks)
		pendingSystemBlocks = nil
		if len(wrapped) == 0 {
			return emit(role, blocks)
		}

		if isNextToolResultUser {
			// 推迟到 contiguous tool_result run 之后；本轮先 emit tool_result 保持位置。
			deferredWrap = append(deferredWrap, wrapped...)
			return emit("user", blocks)
		}

		if role == "user" {
			// 普通 user：prepend wrapped 到 blocks，emit 单条 user message
			merged := make([]dto.ClaudeMediaMessage, 0, len(wrapped)+len(blocks))
			merged = append(merged, wrapped...)
			merged = append(merged, blocks...)
			return emit("user", merged)
		}

		// role == assistant：handshake guard 已在入口拦截，这里只需选择 wrapped 落点。
		if appendToLastUserContent(wrapped) {
			return emit(role, blocks)
		}
		if err := emit("user", wrapped); err != nil {
			return err
		}
		return emit(role, blocks)
	}

	for _, item := range items {
		role, blocks, err := convertResponsesInputItem(item)
		if err != nil {
			return nil, nil, err
		}
		if len(blocks) == 0 {
			continue
		}

		if role == roleSystemSentinel {
			if !seenNonSystem {
				systemPrefixBlocks = append(systemPrefixBlocks, blocks...)
				continue
			}
			pendingSystemBlocks = append(pendingSystemBlocks, blocks...)
			continue
		}

		seenNonSystem = true
		if err := emitWithPendingFlush(role, blocks); err != nil {
			return nil, nil, err
		}
	}

	// 末尾收尾：先 flush 推迟的 wrap（如果整段以 tool_result 结尾会留在 deferred 里）
	if err := flushDeferredWrap(); err != nil {
		return nil, nil, err
	}

	// 末尾未消费的 pending dev：与中间的 assistant 分支同构。
	if len(pendingSystemBlocks) > 0 {
		wrapped := wrapSystemBlocksAsUserText(pendingSystemBlocks)
		pendingSystemBlocks = nil
		if len(wrapped) > 0 {
			if lastAssistantHasToolUse() {
				return nil, nil, errors.New("trailing system/developer cannot follow an unresolved assistant tool_use; provide the matching tool_result first")
			}
			if !appendToLastUserContent(wrapped) {
				if err := emit("user", wrapped); err != nil {
					return nil, nil, err
				}
			}
		}
	}

	if len(messages) == 0 && len(systemPrefixBlocks) == 0 {
		return nil, nil, errors.New("input did not produce any messages")
	}
	if len(messages) == 0 {
		return nil, nil, errors.New("input contained only system/developer messages without any user/assistant turn")
	}
	return messages, systemPrefixBlocks, nil
}

// 把穿插在对话中间的 system/developer text 块包成 <openai_developer_instruction> XML，
// 让 Claude 把它当作显式的开发者指令读取——保留时序但优先级降到 user 文本内嵌。
// 标签前缀加 openai_ 命名空间，降低与用户内容碰撞概率（不是安全边界，仅是约定）。
// 每条 block 单独包一对标签，保留原始 OpenAI message 的边界，便于模型识别多条独立指令。
func wrapSystemBlocksAsUserText(blocks []dto.ClaudeMediaMessage) []dto.ClaudeMediaMessage {
	if len(blocks) == 0 {
		return nil
	}
	out := make([]dto.ClaudeMediaMessage, 0, len(blocks))
	for _, b := range blocks {
		if b.Type != "text" || b.Text == nil || *b.Text == "" {
			continue
		}
		wrapped := "<openai_developer_instruction>\n" + *b.Text + "\n</openai_developer_instruction>"
		out = append(out, dto.ClaudeMediaMessage{
			Type: "text",
			Text: common.GetPointer(wrapped),
		})
	}
	return out
}

// Anthropic 协议要求 thinking/redacted_thinking 必须排在同一 assistant message 的非-thinking 块之前。
// 客户端按 OpenAI Responses 顺序拼回 reasoning item 时，若上一个 assistant message 已经有 text/tool_use，
// 再把 reasoning 追加进去就会违反此约束并被 Claude 拒绝；直接 400 避免静默拼成非法请求。
func assertReasoningOrder(existing, incoming []dto.ClaudeMediaMessage) error {
	hasIncomingReasoning := false
	for _, b := range incoming {
		if b.Type == "thinking" || b.Type == "redacted_thinking" {
			hasIncomingReasoning = true
			break
		}
	}
	if !hasIncomingReasoning {
		return nil
	}
	for _, b := range existing {
		if b.Type != "thinking" && b.Type != "redacted_thinking" {
			return errors.New("reasoning item must precede non-reasoning content within the same assistant turn; reorder your input so reasoning comes before message/function_call items")
		}
	}
	return nil
}

func convertResponsesInputItem(item map[string]any) (role string, blocks []dto.ClaudeMediaMessage, err error) {
	itemType, _ := item["type"].(string)
	switch itemType {
	case "", "message":
		return convertResponsesInputMessage(item)
	case "function_call":
		blk, err := convertResponsesInputFunctionCall(item)
		if err != nil {
			return "", nil, err
		}
		return "assistant", []dto.ClaudeMediaMessage{blk}, nil
	case "function_call_output":
		blk, err := convertResponsesInputFunctionCallOutput(item)
		if err != nil {
			return "", nil, err
		}
		return "user", []dto.ClaudeMediaMessage{blk}, nil
	case "custom_tool_call":
		// 客户端把上一轮我们返回的 custom_tool_call 回传给我们。因为请求侧把 custom tool 降级为
		// {input: string} schema 的 function tool，所以这里要把 raw string 重新包成 {"input": ...}
		// 才与 Anthropic 上游已知的 tool schema 对得上。
		blk, err := convertResponsesInputCustomToolCall(item)
		if err != nil {
			return "", nil, err
		}
		return "assistant", []dto.ClaudeMediaMessage{blk}, nil
	case "custom_tool_call_output":
		blk, err := convertResponsesInputFunctionCallOutput(item)
		if err != nil {
			return "", nil, err
		}
		return "user", []dto.ClaudeMediaMessage{blk}, nil
	case "reasoning":
		blk, err := convertResponsesInputReasoning(item)
		if err != nil {
			return "", nil, err
		}
		if blk == nil {
			return "", nil, nil
		}
		return "assistant", []dto.ClaudeMediaMessage{*blk}, nil
	case "item_reference":
		return "", nil, errors.New("item_reference is not supported when converting to Anthropic Messages API")
	case "web_search_call", "file_search_call", "code_interpreter_call",
		"image_generation_call", "computer_call", "computer_call_output",
		"local_shell_call", "mcp_call", "mcp_list_tools",
		"mcp_approval_request", "mcp_approval_response":
		return "", nil, fmt.Errorf("input item type %q is not supported when converting to Anthropic Messages API", itemType)
	}
	return "", nil, fmt.Errorf("unknown input item type %q", itemType)
}

// roleSystemSentinel 是把 OpenAI Responses 中 role=system/developer 的 message
// 在内部分发流水线里暂存的占位 role。Anthropic Messages API 没有 system role，
// 真正的派发在 convertResponsesInputToClaudeMessages 里：开头连续的抽到顶层 system，
// 穿插在中间的降级成 XML 包裹的 user text。
const roleSystemSentinel = "__system__"

func convertResponsesInputMessage(item map[string]any) (string, []dto.ClaudeMediaMessage, error) {
	role, _ := item["role"].(string)
	if role == "" {
		role = "user"
	}
	switch role {
	case "user", "assistant":
	case "system", "developer":
		// 用 sentinel 把 system/developer 透传到上层，由上层决定抽到 system 还是降级到 user。
		role = roleSystemSentinel
	default:
		return "", nil, fmt.Errorf("unknown message role %q", role)
	}

	content := item["content"]
	if content == nil {
		return role, nil, nil
	}
	if s, ok := content.(string); ok {
		if s == "" {
			return role, nil, nil
		}
		return role, []dto.ClaudeMediaMessage{{
			Type: "text",
			Text: common.GetPointer(s),
		}}, nil
	}

	parts, ok := content.([]any)
	if !ok {
		return "", nil, fmt.Errorf("message content must be string or array, got %T", content)
	}
	blocks := make([]dto.ClaudeMediaMessage, 0, len(parts))
	for _, p := range parts {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		blk, err := convertResponsesContentPart(role, pm)
		if err != nil {
			return "", nil, err
		}
		if blk == nil {
			continue
		}
		// system/developer 消息只允许文本，与 OpenAI Responses 的约束一致；
		// 既避免把图片塞进 Anthropic 顶层 system（不支持），也避免被 XML 包裹后内容失真。
		if role == roleSystemSentinel && blk.Type != "text" {
			return "", nil, fmt.Errorf("system/developer message content must be text only, got %q", blk.Type)
		}
		blocks = append(blocks, *blk)
	}
	return role, blocks, nil
}

func convertResponsesContentPart(role string, part map[string]any) (*dto.ClaudeMediaMessage, error) {
	partType, _ := part["type"].(string)
	switch partType {
	case "input_text", "text":
		text, _ := part["text"].(string)
		if text == "" {
			return nil, nil
		}
		return &dto.ClaudeMediaMessage{Type: "text", Text: common.GetPointer(text)}, nil
	case "output_text":
		text, _ := part["text"].(string)
		if text == "" {
			return nil, nil
		}
		return &dto.ClaudeMediaMessage{Type: "text", Text: common.GetPointer(text)}, nil
	case "refusal":
		text, _ := part["refusal"].(string)
		if text == "" {
			return nil, nil
		}
		return &dto.ClaudeMediaMessage{Type: "text", Text: common.GetPointer(text)}, nil
	case "input_image":
		return convertResponsesInputImage(part)
	case "input_file":
		return convertResponsesInputFile(part)
	case "input_audio":
		return nil, errors.New("input_audio is not supported by Anthropic Messages API")
	}
	return nil, fmt.Errorf("unknown content part type %q", partType)
}

func convertResponsesInputImage(part map[string]any) (*dto.ClaudeMediaMessage, error) {
	src := &dto.ClaudeMessageSource{}
	if url, _ := part["image_url"].(string); url != "" {
		if mediaType, data, ok := parseDataURL(url); ok {
			src.Type = "base64"
			src.MediaType = mediaType
			src.Data = data
		} else {
			src.Type = "url"
			src.Url = url
		}
		return &dto.ClaudeMediaMessage{Type: "image", Source: src}, nil
	}
	if fileID, _ := part["file_id"].(string); fileID != "" {
		return nil, errors.New("input_image by file_id is not supported when converting to Anthropic Messages API")
	}
	return nil, errors.New("input_image requires image_url")
}

func convertResponsesInputFile(part map[string]any) (*dto.ClaudeMediaMessage, error) {
	if url, _ := part["file_url"].(string); url != "" {
		if mediaType, data, ok := parseDataURL(url); ok {
			return &dto.ClaudeMediaMessage{
				Type: "document",
				Source: &dto.ClaudeMessageSource{
					Type:      "base64",
					MediaType: mediaType,
					Data:      data,
				},
			}, nil
		}
		return &dto.ClaudeMediaMessage{
			Type: "document",
			Source: &dto.ClaudeMessageSource{
				Type: "url",
				Url:  url,
			},
		}, nil
	}
	if data, _ := part["file_data"].(string); data != "" {
		mediaType := "application/pdf"
		if mt, ok := part["mime_type"].(string); ok && mt != "" {
			mediaType = mt
		}
		return &dto.ClaudeMediaMessage{
			Type: "document",
			Source: &dto.ClaudeMessageSource{
				Type:      "base64",
				MediaType: mediaType,
				Data:      data,
			},
		}, nil
	}
	return nil, errors.New("input_file requires file_url or file_data")
}

func parseDataURL(url string) (mediaType, data string, ok bool) {
	if !strings.HasPrefix(url, "data:") {
		return "", "", false
	}
	rest := strings.TrimPrefix(url, "data:")
	idx := strings.Index(rest, ";base64,")
	if idx < 0 {
		return "", "", false
	}
	return rest[:idx], rest[idx+len(";base64,"):], true
}

func convertResponsesInputFunctionCall(item map[string]any) (dto.ClaudeMediaMessage, error) {
	callID, _ := item["call_id"].(string)
	name, _ := item["name"].(string)
	if callID == "" || name == "" {
		return dto.ClaudeMediaMessage{}, errors.New("function_call requires call_id and name")
	}
	args := item["arguments"]
	var input any
	switch v := args.(type) {
	case string:
		if v != "" {
			if err := common.UnmarshalJsonStr(v, &input); err != nil {
				input = v
			}
		} else {
			input = map[string]any{}
		}
	case nil:
		input = map[string]any{}
	default:
		input = v
	}
	return dto.ClaudeMediaMessage{
		Type:  "tool_use",
		Id:    callID,
		Name:  name,
		Input: input,
	}, nil
}

func convertResponsesInputCustomToolCall(item map[string]any) (dto.ClaudeMediaMessage, error) {
	callID, _ := item["call_id"].(string)
	name, _ := item["name"].(string)
	if callID == "" || name == "" {
		return dto.ClaudeMediaMessage{}, errors.New("custom_tool_call requires call_id and name")
	}
	input, _ := item["input"].(string)
	return dto.ClaudeMediaMessage{
		Type: "tool_use",
		Id:   callID,
		Name: name,
		Input: map[string]any{
			"input": input,
		},
	}, nil
}

func convertResponsesInputFunctionCallOutput(item map[string]any) (dto.ClaudeMediaMessage, error) {
	callID, _ := item["call_id"].(string)
	if callID == "" {
		return dto.ClaudeMediaMessage{}, errors.New("function_call_output requires call_id")
	}
	out := item["output"]
	var content any
	switch v := out.(type) {
	case string:
		content = v
	case []any:
		content = v
	case map[string]any:
		raw, err := common.Marshal(v)
		if err != nil {
			return dto.ClaudeMediaMessage{}, err
		}
		content = string(raw)
	case nil:
		content = ""
	default:
		raw, err := common.Marshal(v)
		if err != nil {
			return dto.ClaudeMediaMessage{}, err
		}
		content = string(raw)
	}
	return dto.ClaudeMediaMessage{
		Type:      "tool_result",
		ToolUseId: callID,
		Content:   content,
	}, nil
}

// 签名严格依赖 encrypted_content 解出来的值；只有 summary 文字不足以让 Claude 验签通过。
func convertResponsesInputReasoning(item map[string]any) (*dto.ClaudeMediaMessage, error) {
	encrypted, _ := item["encrypted_content"].(string)
	kind, signature, data, err := DecodeReasoningEncryptedContent(encrypted)
	if err != nil {
		return nil, err
	}

	if kind == ReasoningKindRedacted && data != "" {
		return &dto.ClaudeMediaMessage{
			Type: "redacted_thinking",
			Data: data,
		}, nil
	}

	var thinking string
	if summary, ok := item["summary"].([]any); ok {
		parts := make([]string, 0, len(summary))
		for _, s := range summary {
			sm, ok := s.(map[string]any)
			if !ok {
				continue
			}
			if text, _ := sm["text"].(string); text != "" {
				parts = append(parts, text)
			}
		}
		thinking = strings.Join(parts, "")
	}
	if thinking == "" {
		if content, ok := item["content"].([]any); ok {
			parts := make([]string, 0, len(content))
			for _, c := range content {
				cm, ok := c.(map[string]any)
				if !ok {
					continue
				}
				if text, _ := cm["text"].(string); text != "" {
					parts = append(parts, text)
				}
			}
			thinking = strings.Join(parts, "")
		}
	}

	if signature == "" && thinking == "" {
		return nil, nil
	}

	blk := &dto.ClaudeMediaMessage{
		Type:      "thinking",
		Thinking:  common.GetPointer(thinking),
		Signature: common.GetPointer(signature),
	}
	return blk, nil
}

func convertResponsesToolsToClaudeTools(raw []byte) ([]any, map[string]bool, map[string]bool, error) {
	if isJSONNull(raw) {
		return nil, nil, nil, nil
	}
	var tools []map[string]any
	if err := common.Unmarshal(raw, &tools); err != nil {
		return nil, nil, nil, fmt.Errorf("tools must be an array: %w", err)
	}
	result := make([]any, 0, len(tools))
	var customNames map[string]bool
	survivingNames := map[string]bool{}
	for _, t := range tools {
		ty, _ := t["type"].(string)
		switch ty {
		case "function":
			tool, err := convertResponsesFunctionToolToClaude(t)
			if err != nil {
				return nil, nil, nil, err
			}
			if survivingNames[tool.Name] {
				return nil, nil, nil, fmt.Errorf("duplicate tool name %q", tool.Name)
			}
			survivingNames[tool.Name] = true
			result = append(result, tool)
		case "custom":
			tool, err := convertResponsesCustomToolToClaude(t)
			if err != nil {
				return nil, nil, nil, err
			}
			if survivingNames[tool.Name] {
				return nil, nil, nil, fmt.Errorf("duplicate tool name %q (function/custom name conflict cannot be disambiguated when round-tripping through Anthropic)", tool.Name)
			}
			survivingNames[tool.Name] = true
			result = append(result, tool)
			if customNames == nil {
				customNames = map[string]bool{}
			}
			customNames[tool.Name] = true
		case "web_search_preview", "web_search",
			"file_search",
			"code_interpreter",
			"computer_use_preview", "computer",
			"image_generation",
			"mcp":
			// 上游 Anthropic 不支持这些 OpenAI 内置服务端工具（或者支持但需要单独开通/付费/语义不一致），
			// 静默剥离避免转发到上游导致 schema 错误或意外计费；模型不会看到这些工具，行为等价于客户端没传。
			continue
		default:
			return nil, nil, nil, fmt.Errorf("unsupported tool type %q", ty)
		}
	}
	return result, customNames, survivingNames, nil
}

// Anthropic 无 free-text/grammar 输入工具的原生对应；把 OpenAI custom tool 降级为接受单个 input string 的 function tool。
// grammar 约束（lark/regex）作为描述注入，依赖模型自觉遵守，协议层不强制。
func convertResponsesCustomToolToClaude(t map[string]any) (*dto.Tool, error) {
	name, _ := t["name"].(string)
	if name == "" {
		return nil, errors.New("custom tool requires name")
	}
	desc, _ := t["description"].(string)
	// custom tool 在 OpenAI 是 freeform 文本输入，但我们降级成 {input: string} 的 function tool 后
	// 输入会被 JSON 包一层，原描述里「不要用 JSON 包装」之类的提示会与实际协议矛盾，需要剥掉。
	desc = stripFreeformHint(desc)
	if format, ok := t["format"].(map[string]any); ok {
		if ftype, _ := format["type"].(string); ftype == "grammar" {
			syntax, _ := format["syntax"].(string)
			definition, _ := format["definition"].(string)
			if definition != "" {
				const maxGrammarBytes = 8192
				truncated := false
				if len(definition) > maxGrammarBytes {
					definition = definition[:maxGrammarBytes]
					truncated = true
				}
				if desc != "" {
					desc += "\n\n"
				}
				desc += "Input must conform to the following " + syntax + " grammar:\n" + definition
				if truncated {
					desc += "\n[grammar truncated]"
				}
			}
		}
	}
	return &dto.Tool{
		Name:        name,
		Description: desc,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input": map[string]any{
					"type":        "string",
					"description": "The raw input string for this custom tool.",
				},
			},
			"required": []any{"input"},
		},
	}, nil
}

func convertResponsesFunctionToolToClaude(t map[string]any) (*dto.Tool, error) {
	name, _ := t["name"].(string)
	if name == "" {
		return nil, errors.New("function tool requires name")
	}
	desc, _ := t["description"].(string)
	params, _ := t["parameters"].(map[string]any)
	if params == nil {
		params = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	tool := &dto.Tool{
		Name:        name,
		Description: desc,
		InputSchema: make(map[string]any, len(params)),
	}
	for k, v := range params {
		tool.InputSchema[k] = v
	}
	if _, ok := tool.InputSchema["type"]; !ok {
		tool.InputSchema["type"] = "object"
	}
	return tool, nil
}

// Responses tool_choice 与 Chat Completions 不同：function 形态是 {type,name} 而非 {type,function:{name}}。
// 单独实现，不复用 chat 版本的 mapToolChoice。
// 接收 surviving tool name 集合：内置工具被静默剥离后，tool_choice 指向已剥离工具或 required+空 tools
// 都必须 unset，否则 Anthropic 会 400。
func convertResponsesToolChoiceToClaude(raw []byte, survivingTools map[string]bool) (*dto.ClaudeToolChoice, error) {
	if isJSONNull(raw) {
		return nil, nil
	}
	hasSurviving := len(survivingTools) > 0
	var asString string
	if err := common.Unmarshal(raw, &asString); err == nil {
		switch asString {
		case "auto":
			if !hasSurviving {
				return nil, nil
			}
			return &dto.ClaudeToolChoice{Type: "auto"}, nil
		case "required":
			if !hasSurviving {
				return nil, nil
			}
			return &dto.ClaudeToolChoice{Type: "any"}, nil
		case "none":
			return &dto.ClaudeToolChoice{Type: "none"}, nil
		case "":
			return nil, nil
		default:
			return nil, fmt.Errorf("unknown tool_choice %q", asString)
		}
	}
	var asObject map[string]any
	if err := common.Unmarshal(raw, &asObject); err != nil {
		return nil, fmt.Errorf("tool_choice must be string or object: %w", err)
	}
	ty, _ := asObject["type"].(string)
	switch ty {
	case "function", "custom":
		name, _ := asObject["name"].(string)
		if name == "" {
			return nil, errors.New("tool_choice." + ty + " requires name")
		}
		if !survivingTools[name] {
			return nil, nil
		}
		return &dto.ClaudeToolChoice{Type: "tool", Name: name}, nil
	case "allowed_tools":
		return nil, errors.New("tool_choice.allowed_tools is not supported when converting to Anthropic Messages API; downgrading to auto would silently broaden the allowed tool set")
	case "auto", "":
		if !hasSurviving {
			return nil, nil
		}
		return &dto.ClaudeToolChoice{Type: "auto"}, nil
	case "none":
		return &dto.ClaudeToolChoice{Type: "none"}, nil
	}
	return nil, fmt.Errorf("unsupported tool_choice type %q", ty)
}

// Anthropic adaptive thinking 没有 effort 等级，只有 type+display；
// 强度信息在 adaptive 下由模型自决，effort 的低/中/高被吞掉。
func mapResponsesReasoningToClaudeThinking(r *dto.Reasoning) *dto.Thinking {
	if r == nil {
		return nil
	}
	if r.Effort == "minimal" {
		return &dto.Thinking{Type: "disabled"}
	}
	t := &dto.Thinking{Type: "adaptive"}
	switch r.Summary {
	case "none":
		t.Display = "omitted"
	case "auto", "concise", "detailed", "":
		t.Display = "summarized"
	default:
		t.Display = "summarized"
	}
	return t
}

func convertResponsesMetadataToClaude(raw []byte) ([]byte, error) {
	if isJSONNull(raw) {
		return nil, nil
	}
	var meta map[string]any
	if err := common.Unmarshal(raw, &meta); err != nil {
		return nil, err
	}
	if userID, ok := meta["user_id"].(string); ok && userID != "" {
		out, err := common.Marshal(map[string]string{"user_id": userID})
		if err != nil {
			return nil, err
		}
		return out, nil
	}
	return nil, nil
}

// 匹配 OpenAI Codex apply_patch 等 custom tool 描述里「This is a FREEFORM tool, so do not wrap the patch in JSON.」之类的整句。
// 我们已把 custom tool 降级为 {input: string}，再保留这句话会让模型拒绝按 schema 输出。
var freeformHintRE = regexp.MustCompile(`(?i)[^.\n]*\bfreeform\b[^.\n]*(?:\.|\n|$)`)

func stripFreeformHint(desc string) string {
	if desc == "" {
		return desc
	}
	cleaned := freeformHintRE.ReplaceAllString(desc, "")
	return strings.TrimSpace(cleaned)
}
