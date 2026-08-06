package claude

import (
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func ptrStr(s string) *string { return &s }
func ptrInt(i int) *int       { return &i }

func TestStreamStateThinkingTextToolUse(t *testing.T) {
	state := NewClaudeResponsesStreamState("claude-opus-4-7")
	state.CreatedAt = 1700000000
	state.ResponseID = "resp_abc"

	usage := &dto.ClaudeUsage{InputTokens: 50, OutputTokens: 0}
	feed := func(c *dto.ClaudeResponse) []dto.ResponsesStreamResponse {
		return state.HandleClaudeChunk(c)
	}

	all := []dto.ResponsesStreamResponse{}
	all = append(all, feed(&dto.ClaudeResponse{
		Type: "message_start",
		Message: &dto.ClaudeMediaMessage{
			Id:    "msg_xyz",
			Model: "claude-opus-4-7",
			Usage: usage,
		},
	})...)
	all = append(all, feed(&dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        ptrInt(0),
		ContentBlock: &dto.ClaudeMediaMessage{Type: "thinking", Thinking: ptrStr(""), Signature: ptrStr("")},
	})...)
	all = append(all, feed(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: ptrInt(0),
		Delta: &dto.ClaudeMediaMessage{Type: "thinking_delta", Thinking: ptrStr("Let me think.")},
	})...)
	all = append(all, feed(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: ptrInt(0),
		Delta: &dto.ClaudeMediaMessage{Type: "signature_delta", Signature: ptrStr("SIG_RAW")},
	})...)
	all = append(all, feed(&dto.ClaudeResponse{Type: "content_block_stop", Index: ptrInt(0)})...)

	all = append(all, feed(&dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        ptrInt(1),
		ContentBlock: &dto.ClaudeMediaMessage{Type: "text", Text: ptrStr("")},
	})...)
	all = append(all, feed(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: ptrInt(1),
		Delta: &dto.ClaudeMediaMessage{Type: "text_delta", Text: ptrStr("Hello ")},
	})...)
	all = append(all, feed(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: ptrInt(1),
		Delta: &dto.ClaudeMediaMessage{Type: "text_delta", Text: ptrStr("world.")},
	})...)
	all = append(all, feed(&dto.ClaudeResponse{Type: "content_block_stop", Index: ptrInt(1)})...)

	all = append(all, feed(&dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        ptrInt(2),
		ContentBlock: &dto.ClaudeMediaMessage{Type: "tool_use", Id: "toolu_001", Name: "get_weather"},
	})...)
	all = append(all, feed(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: ptrInt(2),
		Delta: &dto.ClaudeMediaMessage{Type: "input_json_delta", PartialJson: ptrStr(`{"city":`)},
	})...)
	all = append(all, feed(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: ptrInt(2),
		Delta: &dto.ClaudeMediaMessage{Type: "input_json_delta", PartialJson: ptrStr(`"SF"}`)},
	})...)
	all = append(all, feed(&dto.ClaudeResponse{Type: "content_block_stop", Index: ptrInt(2)})...)

	all = append(all, feed(&dto.ClaudeResponse{
		Type:  "message_delta",
		Delta: &dto.ClaudeMediaMessage{StopReason: ptrStr("tool_use")},
		Usage: &dto.ClaudeUsage{OutputTokens: 42},
	})...)
	all = append(all, feed(&dto.ClaudeResponse{Type: "message_stop"})...)
	all = append(all, state.FinalEvents()...)

	gotTypes := make([]string, 0, len(all))
	for _, e := range all {
		gotTypes = append(gotTypes, e.Type)
	}
	wantSeq := []string{
		respEventCreated, respEventInProgress,
		respEventOutputItemAdded, respEventReasoningSummaryPartAdded,
		respEventReasoningSummaryTextDelta,
		respEventReasoningSummaryTextDone, respEventReasoningSummaryPartDone, respEventOutputItemDone,
		respEventOutputItemAdded, respEventContentPartAdded,
		respEventOutputTextDelta, respEventOutputTextDelta,
		respEventOutputTextDone, respEventContentPartDone, respEventOutputItemDone,
		respEventOutputItemAdded,
		respEventFnCallArgsDelta, respEventFnCallArgsDelta,
		respEventFnCallArgsDone, respEventOutputItemDone,
		respEventCompleted,
	}
	if len(gotTypes) != len(wantSeq) {
		t.Fatalf("event count mismatch: got %d (%v), want %d (%v)", len(gotTypes), gotTypes, len(wantSeq), wantSeq)
	}
	for i, want := range wantSeq {
		if gotTypes[i] != want {
			t.Errorf("event[%d]: got %q want %q (full: %v)", i, gotTypes[i], want, gotTypes)
		}
	}

	for i, e := range all {
		if e.SequenceNumber != i {
			t.Errorf("event[%d].SequenceNumber=%d want %d", i, e.SequenceNumber, i)
		}
	}

	final := all[len(all)-1]
	if final.Response == nil || len(final.Response.Output) != 3 {
		t.Fatalf("final response should have 3 output items, got %+v", final.Response)
	}
	reasoning := final.Response.Output[0]
	if reasoning.Type != "reasoning" {
		t.Errorf("output[0].type=%q want reasoning", reasoning.Type)
	}
	if reasoning.EncryptedContent == "" {
		t.Error("reasoning encrypted_content should not be empty")
	}
	_, decodedSig, _, _ := DecodeReasoningEncryptedContent(reasoning.EncryptedContent)
	if decodedSig != "SIG_RAW" {
		t.Errorf("encrypted_content signature round-trip: got %q want SIG_RAW", decodedSig)
	}
	if len(reasoning.Summary) != 1 || reasoning.Summary[0].Text != "Let me think." {
		t.Errorf("reasoning.summary=%v want single 'Let me think.'", reasoning.Summary)
	}

	msg := final.Response.Output[1]
	if msg.Type != "message" || len(msg.Content) != 1 || msg.Content[0].Text != "Hello world." {
		t.Errorf("output[1] message wrong: %+v", msg)
	}

	tc := final.Response.Output[2]
	if tc.Type != "function_call" || tc.CallId != "toolu_001" || tc.Name != "get_weather" {
		t.Errorf("output[2] function_call wrong: %+v", tc)
	}
	if string(tc.Arguments) != `"{\"city\":\"SF\"}"` {
		t.Errorf("tool arguments: got %s want quoted JSON string", string(tc.Arguments))
	}
	if got := tc.ArgumentsString(); got != `{"city":"SF"}` {
		t.Errorf("ArgumentsString=%q want {\"city\":\"SF\"}", got)
	}

	// Stream 路径里 response.output_item.done 的 item.arguments 也必须是字符串字面（与 OpenAI 官方一致），
	// 否则 Codex 等客户端 JSON.parse(item.arguments) 会抛错。
	var streamToolDoneItem *dto.ResponsesOutput
	for _, e := range all {
		if e.Type == respEventOutputItemDone && e.Item != nil && e.Item.Type == "function_call" {
			streamToolDoneItem = e.Item
		}
	}
	if streamToolDoneItem == nil {
		t.Fatalf("expected stream output_item.done for function_call")
	}
	rawDoneItem, _ := common.Marshal(streamToolDoneItem)
	if !strings.Contains(string(rawDoneItem), `"arguments":"{\"city\":\"SF\"}"`) {
		t.Errorf("stream done item arguments not serialized as JSON string: %s", string(rawDoneItem))
	}
}

func TestResponsesRequestReasoningRoundTrip(t *testing.T) {
	sig := "SIG_FROM_CLAUDE_PRIOR_TURN"
	encryptedRaw := EncodeThinkingSignature(sig)
	inputJSON := `[
		{"role":"user","content":"hi"},
		{"type":"reasoning","id":"rs_1","encrypted_content":"` + encryptedRaw + `","summary":[{"type":"summary_text","text":"thinking text"}]},
		{"role":"assistant","content":[{"type":"output_text","text":"answer"}]}
	]`
	req := &dto.OpenAIResponsesRequest{
		Model: "claude-opus-4-7",
		Input: []byte(inputJSON),
	}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(claude.Messages) != 2 {
		t.Fatalf("messages=%d want 2", len(claude.Messages))
	}
	assistant := claude.Messages[1]
	if assistant.Role != "assistant" {
		t.Fatalf("messages[1].role=%q want assistant", assistant.Role)
	}
	blocks, _ := assistant.ParseContent()
	if len(blocks) != 2 {
		t.Fatalf("assistant blocks=%d want 2 (thinking + text)", len(blocks))
	}
	if blocks[0].Type != "thinking" {
		t.Errorf("blocks[0].type=%q want thinking", blocks[0].Type)
	}
	if blocks[0].Signature == nil || *blocks[0].Signature != sig {
		t.Errorf("blocks[0].signature=%v want %q", blocks[0].Signature, sig)
	}
	if blocks[0].Thinking == nil || *blocks[0].Thinking != "thinking text" {
		t.Errorf("blocks[0].thinking=%v want 'thinking text'", blocks[0].Thinking)
	}
	if blocks[1].Type != "text" {
		t.Errorf("blocks[1].type=%q want text", blocks[1].Type)
	}
}

func TestResponsesRequestPreviousResponseIDRejected(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:              "claude-opus-4-7",
		Input:              []byte(`"hi"`),
		PreviousResponseID: "resp_prev_xxx",
	}
	_, _, _, err := ConvertResponsesRequestToClaude(req)
	if err == nil || !strings.Contains(err.Error(), "previous_response_id") {
		t.Errorf("expected previous_response_id rejection, got %v", err)
	}
}

func TestResponsesRequestJSONSchemaFormatRejected(t *testing.T) {
	textRaw, _ := common.Marshal(map[string]any{"format": map[string]any{"type": "json_schema"}})
	req := &dto.OpenAIResponsesRequest{
		Model: "claude-opus-4-7",
		Input: []byte(`"hi"`),
		Text:  textRaw,
	}
	_, _, _, err := ConvertResponsesRequestToClaude(req)
	if err == nil || !strings.Contains(err.Error(), "json_schema") {
		t.Errorf("expected json_schema rejection, got %v", err)
	}
}

func TestResponsesRequestReasoningEffortMapsToAdaptive(t *testing.T) {
	cases := []struct {
		name     string
		effort   string
		summary  string
		wantType string
		wantDisp string
	}{
		{"minimal disables", "minimal", "", "disabled", ""},
		{"low to adaptive summarized", "low", "", "adaptive", "summarized"},
		{"medium to adaptive summarized", "medium", "auto", "adaptive", "summarized"},
		{"high to adaptive summarized", "high", "concise", "adaptive", "summarized"},
		{"summary none omits", "high", "none", "adaptive", "omitted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &dto.OpenAIResponsesRequest{
				Model:     "claude-opus-4-7",
				Input:     []byte(`"hi"`),
				Reasoning: &dto.Reasoning{Effort: tc.effort, Summary: tc.summary},
			}
			claude, _, _, err := ConvertResponsesRequestToClaude(req)
			if err != nil {
				t.Fatalf("convert: %v", err)
			}
			if claude.Thinking == nil {
				t.Fatalf("Thinking is nil")
			}
			if claude.Thinking.Type != tc.wantType {
				t.Errorf("Thinking.Type=%q want %q", claude.Thinking.Type, tc.wantType)
			}
			if claude.Thinking.Display != tc.wantDisp {
				t.Errorf("Thinking.Display=%q want %q", claude.Thinking.Display, tc.wantDisp)
			}
		})
	}
}

func TestResponsesRequestToolCallRoundTrip(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"what is the weather in SF?"},
		{"type":"function_call","call_id":"toolu_001","name":"get_weather","arguments":"{\"city\":\"SF\"}"},
		{"type":"function_call_output","call_id":"toolu_001","output":"sunny"}
	]`
	req := &dto.OpenAIResponsesRequest{
		Model: "claude-opus-4-7",
		Input: []byte(inputJSON),
	}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(claude.Messages) != 3 {
		t.Fatalf("messages=%d want 3 (user / assistant tool_use / user tool_result)", len(claude.Messages))
	}
	assistant := claude.Messages[1]
	blocks, _ := assistant.ParseContent()
	if len(blocks) != 1 || blocks[0].Type != "tool_use" || blocks[0].Id != "toolu_001" {
		t.Errorf("assistant blocks wrong: %+v", blocks)
	}
	if blocks[0].Name != "get_weather" {
		t.Errorf("tool name=%q want get_weather", blocks[0].Name)
	}
	inputMap, ok := blocks[0].Input.(map[string]any)
	if !ok || inputMap["city"] != "SF" {
		t.Errorf("tool input=%v want {city:SF}", blocks[0].Input)
	}
	user2 := claude.Messages[2]
	blocks2, _ := user2.ParseContent()
	if len(blocks2) != 1 || blocks2[0].Type != "tool_result" || blocks2[0].ToolUseId != "toolu_001" {
		t.Errorf("tool_result wrong: %+v", blocks2)
	}
	if blocks2[0].Content != "sunny" {
		t.Errorf("tool_result content=%v want sunny", blocks2[0].Content)
	}
}

func TestStreamMaxTokensIncompleteEvent(t *testing.T) {
	state := NewClaudeResponsesStreamState("claude-opus-4-7")
	state.CreatedAt = 1700000000
	state.ResponseID = "resp_x"

	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:    "message_start",
		Message: &dto.ClaudeMediaMessage{Id: "msg_1", Model: "claude-opus-4-7", Usage: &dto.ClaudeUsage{InputTokens: 10}},
	})
	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        ptrInt(0),
		ContentBlock: &dto.ClaudeMediaMessage{Type: "text", Text: ptrStr("")},
	})
	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: ptrInt(0),
		Delta: &dto.ClaudeMediaMessage{Type: "text_delta", Text: ptrStr("partial")},
	})
	state.HandleClaudeChunk(&dto.ClaudeResponse{Type: "content_block_stop", Index: ptrInt(0)})
	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:  "message_delta",
		Delta: &dto.ClaudeMediaMessage{StopReason: ptrStr("max_tokens")},
		Usage: &dto.ClaudeUsage{OutputTokens: 100},
	})
	final := state.FinalEvents()
	if len(final) != 1 || final[0].Type != respEventIncomplete {
		t.Fatalf("final event type=%v want response.incomplete (len=%d)", final, len(final))
	}
	if final[0].Response == nil || final[0].Response.IncompleteDetails == nil {
		t.Fatalf("final response missing incomplete_details: %+v", final[0].Response)
	}
	if final[0].Response.IncompleteDetails.Reason != "max_output_tokens" {
		t.Errorf("incomplete_details.reason=%q want max_output_tokens", final[0].Response.IncompleteDetails.Reason)
	}
	raw, _ := common.Marshal(final[0].Response.IncompleteDetails)
	if !strings.Contains(string(raw), `"reason":"max_output_tokens"`) {
		t.Errorf("serialized JSON should use 'reason' key, got %s", string(raw))
	}
}

func TestNonStreamMaxTokensIncomplete(t *testing.T) {
	cr := &dto.ClaudeResponse{
		Id:         "msg_2",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-opus-4-7",
		StopReason: "max_tokens",
		Content: []dto.ClaudeMediaMessage{
			{Type: "text", Text: ptrStr("partial")},
		},
		Usage: &dto.ClaudeUsage{InputTokens: 5, OutputTokens: 50},
	}
	resp := ConvertClaudeResponseToResponses(cr, nil, nil)
	if resp.IncompleteDetails == nil {
		t.Fatal("incomplete_details should be set")
	}
	if resp.IncompleteDetails.Reason != "max_output_tokens" {
		t.Errorf("reason=%q want max_output_tokens", resp.IncompleteDetails.Reason)
	}
}

func TestAssistantTextThenReasoningReordered(t *testing.T) {
	encryptedRaw := EncodeThinkingSignature("SIG")
	inputJSON := `[
		{"role":"user","content":"hi"},
		{"role":"assistant","content":[{"type":"output_text","text":"hello"}]},
		{"type":"reasoning","id":"rs_1","encrypted_content":"` + encryptedRaw + `","summary":[{"type":"summary_text","text":"thought"}]}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("reasoning after text should be reordered: %v", err)
	}
	if len(claude.Messages) != 2 {
		t.Fatalf("messages=%d want 2", len(claude.Messages))
	}
	blocks, _ := claude.Messages[1].ParseContent()
	if len(blocks) != 2 || blocks[0].Type != "thinking" || blocks[1].Type != "text" {
		t.Errorf("assistant blocks=%v want thinking then text", blocks)
	}
}

func TestAssistantReasoningThenTextAllowed(t *testing.T) {
	encryptedRaw := EncodeThinkingSignature("SIG")
	inputJSON := `[
		{"role":"user","content":"hi"},
		{"type":"reasoning","id":"rs_1","encrypted_content":"` + encryptedRaw + `","summary":[{"type":"summary_text","text":"thought"}]},
		{"role":"assistant","content":[{"type":"output_text","text":"hello"}]}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("should not err: %v", err)
	}
	if len(claude.Messages) != 2 {
		t.Fatalf("messages=%d want 2", len(claude.Messages))
	}
	assistant := claude.Messages[1]
	blocks, _ := assistant.ParseContent()
	if len(blocks) != 2 {
		t.Fatalf("blocks=%d want 2 (thinking+text)", len(blocks))
	}
	if blocks[0].Type != "thinking" || blocks[1].Type != "text" {
		t.Errorf("block order wrong: %s, %s", blocks[0].Type, blocks[1].Type)
	}
}

func TestToolChoiceAllowedToolsRejected(t *testing.T) {
	tc, _ := common.Marshal(map[string]any{"type": "allowed_tools", "tools": []map[string]any{{"type": "function", "name": "x"}}})
	req := &dto.OpenAIResponsesRequest{
		Model:      "claude-opus-4-7",
		Input:      []byte(`"hi"`),
		ToolChoice: tc,
	}
	_, _, _, err := ConvertResponsesRequestToClaude(req)
	if err == nil || !strings.Contains(err.Error(), "allowed_tools") {
		t.Errorf("expected allowed_tools rejection, got %v", err)
	}
}

func TestStreamServerToolUseSkipped(t *testing.T) {
	state := NewClaudeResponsesStreamState("claude-opus-4-7")
	state.CreatedAt = 1700000000
	state.ResponseID = "resp_x"

	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:    "message_start",
		Message: &dto.ClaudeMediaMessage{Id: "msg_1", Model: "claude-opus-4-7", Usage: &dto.ClaudeUsage{InputTokens: 10}},
	})
	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        ptrInt(0),
		ContentBlock: &dto.ClaudeMediaMessage{Type: "server_tool_use", Id: "stu_1", Name: "web_search"},
	})
	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: ptrInt(0),
		Delta: &dto.ClaudeMediaMessage{Type: "input_json_delta", PartialJson: ptrStr(`{"q":`)},
	})
	state.HandleClaudeChunk(&dto.ClaudeResponse{Type: "content_block_stop", Index: ptrInt(0)})

	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        ptrInt(1),
		ContentBlock: &dto.ClaudeMediaMessage{Type: "text", Text: ptrStr("")},
	})
	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: ptrInt(1),
		Delta: &dto.ClaudeMediaMessage{Type: "text_delta", Text: ptrStr("answer")},
	})
	state.HandleClaudeChunk(&dto.ClaudeResponse{Type: "content_block_stop", Index: ptrInt(1)})
	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:  "message_delta",
		Delta: &dto.ClaudeMediaMessage{StopReason: ptrStr("end_turn")},
	})
	final := state.FinalEvents()
	if final[0].Response == nil {
		t.Fatal("final response nil")
	}
	for _, item := range final[0].Response.Output {
		if item.Type == "unknown" {
			t.Errorf("output should not contain type:unknown items, got %+v", item)
		}
	}
	if len(final[0].Response.Output) != 1 {
		t.Errorf("output count=%d want 1 (only text)", len(final[0].Response.Output))
	}
	if len(final[0].Response.Output) >= 1 {
		msg := final[0].Response.Output[0]
		if msg.Type != "message" || msg.Content[0].Text != "answer" {
			t.Errorf("output[0] wrong: %+v", msg)
		}
		want := 1
		if msg.Content[0].Text != "answer" || len(final[0].Response.Output) != want {
			t.Errorf("expected only the text message at output[0], got %+v", final[0].Response.Output)
		}
	}
}

func TestStreamMalformedEnvelopeInInputRejected(t *testing.T) {
	bad := reasoningEnvelopePrefix + "not-base64!!!"
	inputJSON := `[
		{"role":"user","content":"hi"},
		{"type":"reasoning","id":"rs_1","encrypted_content":"` + bad + `","summary":[{"type":"summary_text","text":"x"}]}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	_, _, _, err := ConvertResponsesRequestToClaude(req)
	if err == nil || !strings.Contains(err.Error(), "envelope") {
		t.Errorf("expected envelope decode error, got %v", err)
	}
}

func TestStreamInterleavedThinkingText(t *testing.T) {
	state := NewClaudeResponsesStreamState("claude-opus-4-7")
	state.CreatedAt = 1700000000
	state.ResponseID = "resp_x"

	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:    "message_start",
		Message: &dto.ClaudeMediaMessage{Id: "msg_1", Model: "claude-opus-4-7", Usage: &dto.ClaudeUsage{InputTokens: 10}},
	})

	for i, blockType := range []string{"thinking", "text", "thinking", "text"} {
		state.HandleClaudeChunk(&dto.ClaudeResponse{
			Type:         "content_block_start",
			Index:        ptrInt(i),
			ContentBlock: &dto.ClaudeMediaMessage{Type: blockType, Text: ptrStr(""), Thinking: ptrStr(""), Signature: ptrStr("")},
		})
		deltaType := "text_delta"
		var delta dto.ClaudeMediaMessage
		if blockType == "thinking" {
			deltaType = "thinking_delta"
			delta = dto.ClaudeMediaMessage{Type: deltaType, Thinking: ptrStr("t" + strconv.Itoa(i))}
		} else {
			delta = dto.ClaudeMediaMessage{Type: deltaType, Text: ptrStr("x" + strconv.Itoa(i))}
		}
		state.HandleClaudeChunk(&dto.ClaudeResponse{Type: "content_block_delta", Index: ptrInt(i), Delta: &delta})
		if blockType == "thinking" {
			state.HandleClaudeChunk(&dto.ClaudeResponse{
				Type:  "content_block_delta",
				Index: ptrInt(i),
				Delta: &dto.ClaudeMediaMessage{Type: "signature_delta", Signature: ptrStr("S" + strconv.Itoa(i))},
			})
		}
		state.HandleClaudeChunk(&dto.ClaudeResponse{Type: "content_block_stop", Index: ptrInt(i)})
	}
	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:  "message_delta",
		Delta: &dto.ClaudeMediaMessage{StopReason: ptrStr("end_turn")},
	})
	final := state.FinalEvents()
	if len(final[0].Response.Output) != 4 {
		t.Fatalf("output count=%d want 4 (interleaved)", len(final[0].Response.Output))
	}
	wantTypes := []string{"reasoning", "message", "reasoning", "message"}
	for i, item := range final[0].Response.Output {
		if item.Type != wantTypes[i] {
			t.Errorf("output[%d].type=%q want %q", i, item.Type, wantTypes[i])
		}
	}
	r0 := final[0].Response.Output[0]
	if r0.EncryptedContent == "" {
		t.Error("reasoning[0].encrypted_content empty")
	}
	_, sig, _, err := DecodeReasoningEncryptedContent(r0.EncryptedContent)
	if err != nil || sig != "S0" {
		t.Errorf("reasoning[0] sig roundtrip=%q want S0 err=%v", sig, err)
	}
	r2 := final[0].Response.Output[2]
	_, sig2, _, _ := DecodeReasoningEncryptedContent(r2.EncryptedContent)
	if sig2 != "S2" {
		t.Errorf("reasoning[2] sig=%q want S2", sig2)
	}
}

func TestStreamRedactedThinkingEmitsEncryptedContent(t *testing.T) {
	state := NewClaudeResponsesStreamState("claude-opus-4-7")
	state.CreatedAt = 1700000000
	state.ResponseID = "resp_x"

	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:    "message_start",
		Message: &dto.ClaudeMediaMessage{Id: "msg_1", Model: "claude-opus-4-7", Usage: &dto.ClaudeUsage{InputTokens: 10}},
	})
	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        ptrInt(0),
		ContentBlock: &dto.ClaudeMediaMessage{Type: "redacted_thinking", Data: "REDACTED_BLOB"},
	})
	state.HandleClaudeChunk(&dto.ClaudeResponse{Type: "content_block_stop", Index: ptrInt(0)})
	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:  "message_delta",
		Delta: &dto.ClaudeMediaMessage{StopReason: ptrStr("end_turn")},
	})
	final := state.FinalEvents()
	if len(final[0].Response.Output) != 1 {
		t.Fatalf("output count=%d want 1", len(final[0].Response.Output))
	}
	out := final[0].Response.Output[0]
	if out.Type != "reasoning" {
		t.Errorf("type=%q want reasoning", out.Type)
	}
	if out.EncryptedContent == "" {
		t.Fatal("encrypted_content empty")
	}
	kind, _, data, err := DecodeReasoningEncryptedContent(out.EncryptedContent)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if kind != ReasoningKindRedacted || data != "REDACTED_BLOB" {
		t.Errorf("roundtrip kind=%q data=%q want redacted/REDACTED_BLOB", kind, data)
	}
}

func TestCustomToolDescriptionStripsFreeformHint(t *testing.T) {
	toolsRaw, _ := common.Marshal([]map[string]any{{
		"type":        "custom",
		"name":        "apply_patch",
		"description": "Use the `apply_patch` tool to edit files. This is a FREEFORM tool, so do not wrap the patch in JSON.\n\nInput must conform to the following lark grammar:\nstart: ...",
	}})
	req := &dto.OpenAIResponsesRequest{
		Model: "claude-opus-4-7",
		Input: []byte(`"hi"`),
		Tools: toolsRaw,
	}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	tool := claude.Tools.([]any)[0].(*dto.Tool)
	lower := strings.ToLower(tool.Description)
	if strings.Contains(lower, "freeform") {
		t.Errorf("freeform hint not stripped: %q", tool.Description)
	}
	if strings.Contains(tool.Description, "do not wrap the patch in JSON") {
		t.Errorf("JSON-warning sentence not stripped: %q", tool.Description)
	}
	if !strings.Contains(tool.Description, "apply_patch") || !strings.Contains(tool.Description, "lark grammar") {
		t.Errorf("legitimate description content removed: %q", tool.Description)
	}
}

func TestCustomToolConvertedToFunctionTool(t *testing.T) {
	toolsRaw, _ := common.Marshal([]map[string]any{{
		"type":        "custom",
		"name":        "my_custom",
		"description": "do something",
		"format": map[string]any{
			"type":       "grammar",
			"syntax":     "regex",
			"definition": "^[a-z]+$",
		},
	}})
	req := &dto.OpenAIResponsesRequest{
		Model: "claude-opus-4-7",
		Input: []byte(`"hi"`),
		Tools: toolsRaw,
	}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(claude.Tools.([]any)) != 1 {
		t.Fatalf("tools count=%d want 1", len(claude.Tools.([]any)))
	}
	tool := claude.Tools.([]any)[0].(*dto.Tool)
	if tool.Name != "my_custom" {
		t.Errorf("name=%q want my_custom", tool.Name)
	}
	if !strings.Contains(tool.Description, "regex grammar") || !strings.Contains(tool.Description, "^[a-z]+$") {
		t.Errorf("description missing grammar info: %q", tool.Description)
	}
	props, _ := tool.InputSchema["properties"].(map[string]any)
	if _, ok := props["input"]; !ok {
		t.Errorf("custom tool should have 'input' property, got %+v", tool.InputSchema)
	}
}

func TestNonStreamCustomToolCallRestoredWithNames(t *testing.T) {
	cr := &dto.ClaudeResponse{
		Id:    "msg_x",
		Model: "claude-opus-4-7",
		Role:  "assistant",
		Type:  "message",
		Content: []dto.ClaudeMediaMessage{
			{Type: "tool_use", Id: "toolu_apply", Name: "apply_patch", Input: map[string]any{"input": "*** Begin Patch\n*** End Patch\n"}},
			{Type: "tool_use", Id: "toolu_exec", Name: "exec_command", Input: map[string]any{"cmd": "ls"}},
		},
		StopReason: "tool_use",
		Usage:      &dto.ClaudeUsage{InputTokens: 10, OutputTokens: 20},
	}
	resp := ConvertClaudeResponseToResponses(cr, map[string]bool{"apply_patch": true}, nil)
	if len(resp.Output) != 2 {
		t.Fatalf("output count=%d want 2", len(resp.Output))
	}
	if resp.Output[0].Type != "custom_tool_call" {
		t.Errorf("output[0].type=%q want custom_tool_call", resp.Output[0].Type)
	}
	if resp.Output[0].Input != "*** Begin Patch\n*** End Patch\n" {
		t.Errorf("output[0].input=%q want raw patch text", resp.Output[0].Input)
	}
	if resp.Output[1].Type != "function_call" {
		t.Errorf("output[1].type=%q want function_call", resp.Output[1].Type)
	}
}

func TestStreamCustomToolCallTransparentRoundTrip(t *testing.T) {
	state := NewClaudeResponsesStreamState("claude-opus-4-7")
	state.CreatedAt = 1700000000
	state.ResponseID = "resp_x"
	state.CustomToolNames = map[string]bool{"apply_patch": true}

	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:    "message_start",
		Message: &dto.ClaudeMediaMessage{Id: "msg_1", Model: "claude-opus-4-7", Usage: &dto.ClaudeUsage{InputTokens: 10}},
	})
	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        ptrInt(0),
		ContentBlock: &dto.ClaudeMediaMessage{Type: "tool_use", Id: "toolu_001", Name: "apply_patch"},
	})
	events1 := state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: ptrInt(0),
		Delta: &dto.ClaudeMediaMessage{Type: "input_json_delta", PartialJson: ptrStr(`{"input":"abc`)},
	})
	for _, e := range events1 {
		if e.Type == respEventFnCallArgsDelta {
			t.Errorf("custom tool should not emit function_call_arguments.delta, got %+v", e)
		}
	}
	deltaSeen := ""
	for _, e := range events1 {
		if e.Type == respEventCustomToolInputDelta {
			deltaSeen += e.Delta
		}
	}
	if deltaSeen != "abc" {
		t.Errorf("first chunk delta=%q want abc", deltaSeen)
	}

	events2 := state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: ptrInt(0),
		Delta: &dto.ClaudeMediaMessage{Type: "input_json_delta", PartialJson: ptrStr(`def"}`)},
	})
	for _, e := range events2 {
		if e.Type == respEventCustomToolInputDelta {
			deltaSeen += e.Delta
		}
	}
	if deltaSeen != "abcdef" {
		t.Errorf("after second chunk delta accum=%q want abcdef", deltaSeen)
	}

	stopEvents := state.HandleClaudeChunk(&dto.ClaudeResponse{Type: "content_block_stop", Index: ptrInt(0)})
	state.HandleClaudeChunk(&dto.ClaudeResponse{
		Type:  "message_delta",
		Delta: &dto.ClaudeMediaMessage{StopReason: ptrStr("tool_use")},
	})
	final := state.FinalEvents()

	gotTypes := []string{}
	for _, e := range stopEvents {
		gotTypes = append(gotTypes, e.Type)
	}
	wantInStop := []string{respEventCustomToolInputDone, respEventOutputItemDone}
	for _, want := range wantInStop {
		found := false
		for _, got := range gotTypes {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("stop events missing %q, got %v", want, gotTypes)
		}
	}
	for _, e := range stopEvents {
		if e.Type == respEventCustomToolInputDelta {
			t.Errorf("custom_tool_call_input.delta should not appear in stop events (streamed earlier), got %+v", e)
		}
		if e.Type == respEventCustomToolInputDone {
			if e.Input != "abcdef" {
				t.Errorf("custom_tool_call_input.done.input=%q want abcdef", e.Input)
			}
		}
	}
	if final[0].Response == nil || len(final[0].Response.Output) != 1 {
		t.Fatalf("final output count wrong: %+v", final[0].Response)
	}
	out := final[0].Response.Output[0]
	if out.Type != "custom_tool_call" {
		t.Errorf("final output[0].type=%q want custom_tool_call", out.Type)
	}
	if out.Input != "abcdef" {
		t.Errorf("final output[0].input=%q want abcdef", out.Input)
	}
	if out.Name != "apply_patch" {
		t.Errorf("final output[0].name=%q want apply_patch", out.Name)
	}
}

func TestBuiltinToolsStrippedSilently(t *testing.T) {
	toolsRaw, _ := common.Marshal([]map[string]any{
		{"type": "function", "name": "exec", "parameters": map[string]any{"type": "object"}},
		{"type": "web_search", "external_web_access": true},
		{"type": "file_search"},
		{"type": "code_interpreter"},
		{"type": "image_generation"},
		{"type": "mcp"},
		{"type": "tool_search", "execution": "client"},
	})
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(`"hi"`), Tools: toolsRaw}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	tools := claude.Tools.([]any)
	if len(tools) != 1 {
		t.Errorf("tools count=%d want 1 (only function should survive)", len(tools))
	}
	if fn, ok := tools[0].(*dto.Tool); !ok || fn.Name != "exec" {
		t.Errorf("surviving tool wrong: %+v", tools[0])
	}
}

func TestToolSearchCallAndOutputSilentlySkipped(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"hi"},
		{"type":"tool_search_call","id":"ts_1","status":"completed","queries":["search"]},
		{"type":"tool_search_output","id":"tso_1","tool_search_call_id":"ts_1","tools":[]},
		{"role":"assistant","content":[{"type":"output_text","text":"ok"}]}
	]`
	req := &dto.OpenAIResponsesRequest{
		Model: "claude-opus-4-7",
		Input: []byte(inputJSON),
	}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(claude.Messages) != 2 {
		t.Fatalf("expected 2 messages (user + assistant, tool_search items skipped), got %d", len(claude.Messages))
	}
	if claude.Messages[0].Role != "user" || claude.Messages[1].Role != "assistant" {
		t.Errorf("roles wrong: %q, %q", claude.Messages[0].Role, claude.Messages[1].Role)
	}
}

func TestDeferLoadingIgnoredFunctionLoadsImmediately(t *testing.T) {
	toolsRaw, _ := common.Marshal([]map[string]any{
		{"type": "function", "name": "search_docs", "defer_loading": true, "parameters": map[string]any{"type": "object"}},
		{"type": "function", "name": "exec", "parameters": map[string]any{"type": "object"}},
		{"type": "tool_search", "execution": "server"},
	})
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(`"hi"`), Tools: toolsRaw}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	tools := claude.Tools.([]any)
	if len(tools) != 2 {
		t.Fatalf("expected 2 surviving tools (defer_loading must be ignored, tool_search stripped), got %d: %+v", len(tools), tools)
	}
	names := map[string]bool{}
	for _, x := range tools {
		if fn, ok := x.(*dto.Tool); ok {
			names[fn.Name] = true
		}
	}
	if !names["search_docs"] || !names["exec"] {
		t.Errorf("expected both search_docs and exec to survive, got %+v", names)
	}
}

func TestRequestAcceptsCustomToolCallEcho(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"hi"},
		{"type":"custom_tool_call","status":"completed","call_id":"call_X","name":"apply_patch","input":"*** Begin Patch\n*** Add File: a.txt\n+hi\n*** End Patch\n"},
		{"type":"custom_tool_call_output","call_id":"call_X","output":"Success"}
	]`
	req := &dto.OpenAIResponsesRequest{
		Model: "claude-opus-4-7",
		Input: []byte(inputJSON),
	}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(claude.Messages) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(claude.Messages))
	}
	asst := claude.Messages[1]
	if asst.Role != "assistant" {
		t.Fatalf("messages[1].role=%q want assistant", asst.Role)
	}
	contents, _ := asst.ParseContent()
	if len(contents) != 1 || contents[0].Type != "tool_use" || contents[0].Name != "apply_patch" || contents[0].Id != "call_X" {
		t.Fatalf("expected tool_use(apply_patch, call_X), got %+v", contents)
	}
	inputMap, ok := contents[0].Input.(map[string]any)
	if !ok {
		t.Fatalf("input not a map: %T", contents[0].Input)
	}
	if got, _ := inputMap["input"].(string); got != "*** Begin Patch\n*** Add File: a.txt\n+hi\n*** End Patch\n" {
		t.Errorf("input.input=%q want raw patch text", got)
	}

	user := claude.Messages[2]
	if user.Role != "user" {
		t.Fatalf("messages[2].role=%q want user", user.Role)
	}
	userContents, _ := user.ParseContent()
	if len(userContents) != 1 || userContents[0].Type != "tool_result" || userContents[0].ToolUseId != "call_X" {
		t.Fatalf("expected tool_result(call_X), got %+v", userContents)
	}
}

func TestCustomToolNamesTracked(t *testing.T) {
	toolsRaw, _ := common.Marshal([]map[string]any{
		{"type": "function", "name": "exec"},
		{"type": "custom", "name": "apply_patch"},
		{"type": "custom", "name": "freeform"},
	})
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(`"hi"`), Tools: toolsRaw}
	_, names, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(names) != 2 || !names["apply_patch"] || !names["freeform"] {
		t.Errorf("customNames=%v want {apply_patch:true, freeform:true}", names)
	}
}

func TestDuplicateToolNamesRejected(t *testing.T) {
	cases := []struct {
		name string
		tools []map[string]any
	}{
		{"function-function dup", []map[string]any{
			{"type": "function", "name": "x", "parameters": map[string]any{"type": "object"}},
			{"type": "function", "name": "x", "parameters": map[string]any{"type": "object"}},
		}},
		{"function-custom dup", []map[string]any{
			{"type": "function", "name": "x", "parameters": map[string]any{"type": "object"}},
			{"type": "custom", "name": "x"},
		}},
		{"custom-custom dup", []map[string]any{
			{"type": "custom", "name": "x"},
			{"type": "custom", "name": "x"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := common.Marshal(tc.tools)
			req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(`"hi"`), Tools: raw}
			_, _, _, err := ConvertResponsesRequestToClaude(req)
			if err == nil || !strings.Contains(err.Error(), "duplicate tool name") {
				t.Errorf("expected duplicate name rejection, got %v", err)
			}
		})
	}
}

func TestToolChoiceSanitizedAfterStrip(t *testing.T) {
	// tool_choice 指向被剥离的工具应该被 unset
	t.Run("function tool_choice pointing to stripped web_search", func(t *testing.T) {
		toolsRaw, _ := common.Marshal([]map[string]any{
			{"type": "function", "name": "exec", "parameters": map[string]any{"type": "object"}},
			{"type": "web_search", "external_web_access": true},
		})
		tc, _ := common.Marshal(map[string]any{"type": "function", "name": "web_search"})
		req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(`"hi"`), Tools: toolsRaw, ToolChoice: tc}
		claude, _, _, err := ConvertResponsesRequestToClaude(req)
		if err != nil {
			t.Fatalf("convert: %v", err)
		}
		if claude.ToolChoice != nil {
			t.Errorf("ToolChoice should be unset when pointing to stripped tool, got %+v", claude.ToolChoice)
		}
	})

	t.Run("required with all tools stripped", func(t *testing.T) {
		toolsRaw, _ := common.Marshal([]map[string]any{
			{"type": "web_search", "external_web_access": true},
			{"type": "file_search"},
		})
		tc, _ := common.Marshal("required")
		req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(`"hi"`), Tools: toolsRaw, ToolChoice: tc}
		claude, _, _, err := ConvertResponsesRequestToClaude(req)
		if err != nil {
			t.Fatalf("convert: %v", err)
		}
		if claude.ToolChoice != nil {
			t.Errorf("ToolChoice 'required' should be unset when no surviving tools, got %+v", claude.ToolChoice)
		}
	})

	t.Run("required with surviving tools keeps any", func(t *testing.T) {
		toolsRaw, _ := common.Marshal([]map[string]any{
			{"type": "function", "name": "exec", "parameters": map[string]any{"type": "object"}},
			{"type": "web_search", "external_web_access": true},
		})
		tc, _ := common.Marshal("required")
		req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(`"hi"`), Tools: toolsRaw, ToolChoice: tc}
		claude, _, _, err := ConvertResponsesRequestToClaude(req)
		if err != nil {
			t.Fatalf("convert: %v", err)
		}
		ctc, _ := claude.ToolChoice.(*dto.ClaudeToolChoice)
		if ctc == nil || ctc.Type != "any" {
			t.Errorf("ToolChoice should be 'any' when surviving tools exist, got %+v", claude.ToolChoice)
		}
	})

	t.Run("function tool_choice pointing to surviving function", func(t *testing.T) {
		toolsRaw, _ := common.Marshal([]map[string]any{
			{"type": "function", "name": "exec", "parameters": map[string]any{"type": "object"}},
		})
		tc, _ := common.Marshal(map[string]any{"type": "function", "name": "exec"})
		req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(`"hi"`), Tools: toolsRaw, ToolChoice: tc}
		claude, _, _, err := ConvertResponsesRequestToClaude(req)
		if err != nil {
			t.Fatalf("convert: %v", err)
		}
		ctc, _ := claude.ToolChoice.(*dto.ClaudeToolChoice)
		if ctc == nil || ctc.Type != "tool" || ctc.Name != "exec" {
			t.Errorf("ToolChoice wrong: %+v", claude.ToolChoice)
		}
	})
}

func TestNonStreamCustomToolInputFallbackForMalformedOutput(t *testing.T) {
	cr := &dto.ClaudeResponse{
		Id:    "msg_x",
		Model: "claude-opus-4-7",
		Role:  "assistant",
		Type:  "message",
		Content: []dto.ClaudeMediaMessage{
			// 模型没遵守 schema：用了 text 而不是 input
			{Type: "tool_use", Id: "toolu_a", Name: "apply_patch", Input: map[string]any{"text": "should not lose"}},
		},
		StopReason: "tool_use",
	}
	resp := ConvertClaudeResponseToResponses(cr, map[string]bool{"apply_patch": true}, nil)
	if len(resp.Output) != 1 {
		t.Fatalf("output count=%d want 1", len(resp.Output))
	}
	out := resp.Output[0]
	if out.Type != "custom_tool_call" {
		t.Errorf("type=%q want custom_tool_call", out.Type)
	}
	if !strings.Contains(out.Input, "should not lose") {
		t.Errorf("input=%q should contain raw JSON fallback when schema deviates", out.Input)
	}
}

func TestCustomToolGrammarDescriptionTruncated(t *testing.T) {
	huge := strings.Repeat("x", 16000)
	toolsRaw, _ := common.Marshal([]map[string]any{{
		"type": "custom",
		"name": "freeform",
		"format": map[string]any{
			"type":       "grammar",
			"syntax":     "lark",
			"definition": huge,
		},
	}})
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(`"hi"`), Tools: toolsRaw}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	tool := claude.Tools.([]any)[0].(*dto.Tool)
	if len(tool.Description) > 10000 {
		t.Errorf("description should be truncated, got len=%d", len(tool.Description))
	}
	if !strings.Contains(tool.Description, "[grammar truncated]") {
		t.Errorf("description should contain truncation marker, got %q", tool.Description[:200])
	}
}

func TestDeveloperRolePrefixLiftedToSystem(t *testing.T) {
	inputJSON := `[
		{"role":"developer","content":"you are a translator"},
		{"role":"system","content":"target language: Japanese"},
		{"role":"user","content":"Hello"}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	sysBlocks, ok := claude.System.([]dto.ClaudeMediaMessage)
	if !ok {
		t.Fatalf("System type=%T want []ClaudeMediaMessage", claude.System)
	}
	if len(sysBlocks) != 2 {
		t.Fatalf("system blocks=%d want 2", len(sysBlocks))
	}
	if sysBlocks[0].Text == nil || *sysBlocks[0].Text != "you are a translator" {
		t.Errorf("system[0]=%v want 'you are a translator'", sysBlocks[0].Text)
	}
	if sysBlocks[1].Text == nil || *sysBlocks[1].Text != "target language: Japanese" {
		t.Errorf("system[1]=%v want 'target language: Japanese'", sysBlocks[1].Text)
	}

	if len(claude.Messages) != 1 {
		t.Fatalf("messages=%d want 1 (only user)", len(claude.Messages))
	}
	if claude.Messages[0].Role != "user" {
		t.Errorf("messages[0].role=%q want user", claude.Messages[0].Role)
	}
	userBlocks, _ := claude.Messages[0].ParseContent()
	if len(userBlocks) != 1 || userBlocks[0].Text == nil || *userBlocks[0].Text != "Hello" {
		t.Errorf("user content wrong: %+v", userBlocks)
	}
}

func TestDeveloperRolePrefixMergedWithInstructions(t *testing.T) {
	inputJSON := `[
		{"role":"developer","content":"extra rule"},
		{"role":"user","content":"Hi"}
	]`
	req := &dto.OpenAIResponsesRequest{
		Model:        "claude-opus-4-7",
		Input:        []byte(inputJSON),
		Instructions: []byte(`"base instructions"`),
	}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	sysBlocks, ok := claude.System.([]dto.ClaudeMediaMessage)
	if !ok {
		t.Fatalf("System type=%T want []ClaudeMediaMessage", claude.System)
	}
	if len(sysBlocks) != 2 {
		t.Fatalf("system blocks=%d want 2", len(sysBlocks))
	}
	if *sysBlocks[0].Text != "base instructions" || *sysBlocks[1].Text != "extra rule" {
		t.Errorf("system blocks order wrong: %q, %q", *sysBlocks[0].Text, *sysBlocks[1].Text)
	}
}

func TestDeveloperRoleInterleavedWrappedToNextUser(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"Hello"},
		{"role":"assistant","content":[{"type":"output_text","text":"Hi there"}]},
		{"role":"developer","content":"switch to formal tone"},
		{"role":"user","content":"How are you?"}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if claude.System != nil {
		t.Errorf("System should be nil when no prefix developer, got %v", claude.System)
	}
	if len(claude.Messages) != 3 {
		t.Fatalf("messages=%d want 3 (user, assistant, user)", len(claude.Messages))
	}

	lastUser := claude.Messages[2]
	if lastUser.Role != "user" {
		t.Fatalf("messages[2].role=%q want user", lastUser.Role)
	}
	blocks, _ := lastUser.ParseContent()
	if len(blocks) != 2 {
		t.Fatalf("last user blocks=%d want 2 (wrapped dev + original text)", len(blocks))
	}
	if blocks[0].Text == nil || !strings.Contains(*blocks[0].Text, "<openai_developer_instruction>") || !strings.Contains(*blocks[0].Text, "switch to formal tone") {
		t.Errorf("blocks[0] should wrap developer instruction, got %v", blocks[0].Text)
	}
	if blocks[1].Text == nil || *blocks[1].Text != "How are you?" {
		t.Errorf("blocks[1] should be original user text, got %v", blocks[1].Text)
	}
}

func TestDeveloperRoleTrailingAppendedToLastUser(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"Translate hello"},
		{"role":"developer","content":"to French"}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if len(claude.Messages) != 1 {
		t.Fatalf("messages=%d want 1 (merged into last user)", len(claude.Messages))
	}
	blocks, _ := claude.Messages[0].ParseContent()
	if len(blocks) != 2 {
		t.Fatalf("user blocks=%d want 2 (original + wrapped dev)", len(blocks))
	}
	if *blocks[0].Text != "Translate hello" {
		t.Errorf("blocks[0] wrong: %q", *blocks[0].Text)
	}
	if !strings.Contains(*blocks[1].Text, "<openai_developer_instruction>") || !strings.Contains(*blocks[1].Text, "to French") {
		t.Errorf("blocks[1] should be wrapped developer, got %q", *blocks[1].Text)
	}
}

func TestDeveloperRoleTrailingAfterAssistantBecomesNewUser(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"Hi"},
		{"role":"assistant","content":[{"type":"output_text","text":"Hello"}]},
		{"role":"developer","content":"reply more verbosely next time"}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if len(claude.Messages) != 3 {
		t.Fatalf("messages=%d want 3 (user, assistant, new user carrying dev)", len(claude.Messages))
	}
	if claude.Messages[2].Role != "user" {
		t.Errorf("messages[2].role=%q want user", claude.Messages[2].Role)
	}
	blocks, _ := claude.Messages[2].ParseContent()
	if len(blocks) != 1 || !strings.Contains(*blocks[0].Text, "<openai_developer_instruction>") {
		t.Errorf("trailing dev should be wrapped in new user message, got %+v", blocks)
	}
}

func TestDeveloperRoleOnlySystemNoUserRejected(t *testing.T) {
	inputJSON := `[
		{"role":"developer","content":"just rules"},
		{"role":"system","content":"more rules"}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	_, _, _, err := ConvertResponsesRequestToClaude(req)
	if err == nil || !strings.Contains(err.Error(), "system/developer") {
		t.Errorf("expected rejection for system-only input, got %v", err)
	}
}

func TestDeveloperRoleWithImageContentRejected(t *testing.T) {
	inputJSON := `[
		{"role":"developer","content":[{"type":"input_image","image_url":"https://example.com/a.png"}]},
		{"role":"user","content":"hi"}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	_, _, _, err := ConvertResponsesRequestToClaude(req)
	if err == nil || !strings.Contains(err.Error(), "text only") {
		t.Errorf("expected rejection for non-text content in system/developer, got %v", err)
	}
}

// Oracle round-2 gap: user → developer → assistant should put wrapper BEFORE assistant,
// not after it (which the original trailing-fallback bug would have done).
func TestDeveloperRoleBetweenUserAndAssistant(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"Hello"},
		{"role":"developer","content":"be formal"},
		{"role":"assistant","content":[{"type":"output_text","text":"Greetings"}]}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(claude.Messages) != 2 {
		t.Fatalf("messages=%d want 2 (user-with-wrapper, assistant)", len(claude.Messages))
	}
	if claude.Messages[0].Role != "user" || claude.Messages[1].Role != "assistant" {
		t.Fatalf("roles wrong: %s, %s", claude.Messages[0].Role, claude.Messages[1].Role)
	}
	userBlocks, _ := claude.Messages[0].ParseContent()
	if len(userBlocks) != 2 {
		t.Fatalf("user blocks=%d want 2 (Hello + wrapped dev)", len(userBlocks))
	}
	if *userBlocks[0].Text != "Hello" {
		t.Errorf("user[0]=%q want Hello", *userBlocks[0].Text)
	}
	if !strings.Contains(*userBlocks[1].Text, "<openai_developer_instruction>") || !strings.Contains(*userBlocks[1].Text, "be formal") {
		t.Errorf("user[1] should be wrapped developer, got %q", *userBlocks[1].Text)
	}
}

// Oracle round-2 gap: developer between two assistants must split them, not merge.
func TestDeveloperRoleSplitsTwoAssistants(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"Q1"},
		{"role":"assistant","content":[{"type":"output_text","text":"A1"}]},
		{"role":"developer","content":"switch tone"},
		{"role":"assistant","content":[{"type":"output_text","text":"A2"}]}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(claude.Messages) != 4 {
		t.Fatalf("messages=%d want 4 (user, A1, user-carrier, A2)", len(claude.Messages))
	}
	roles := []string{claude.Messages[0].Role, claude.Messages[1].Role, claude.Messages[2].Role, claude.Messages[3].Role}
	wantRoles := []string{"user", "assistant", "user", "assistant"}
	for i, r := range roles {
		if r != wantRoles[i] {
			t.Errorf("messages[%d].role=%q want %q", i, r, wantRoles[i])
		}
	}
	a1Blocks, _ := claude.Messages[1].ParseContent()
	if len(a1Blocks) != 1 || *a1Blocks[0].Text != "A1" {
		t.Errorf("A1 should remain alone, got %+v", a1Blocks)
	}
	carrierBlocks, _ := claude.Messages[2].ParseContent()
	if len(carrierBlocks) != 1 || !strings.Contains(*carrierBlocks[0].Text, "switch tone") {
		t.Errorf("user-carrier should hold wrapped developer, got %+v", carrierBlocks)
	}
	a2Blocks, _ := claude.Messages[3].ParseContent()
	if len(a2Blocks) != 1 || *a2Blocks[0].Text != "A2" {
		t.Errorf("A2 should remain alone, got %+v", a2Blocks)
	}
}

// Oracle round-2 gap: developer between assistant_text and reasoning shouldn't trigger false
// assertReasoningOrder failure that would happen if both assistants merged.
func TestDeveloperRoleBetweenAssistantTextAndReasoning(t *testing.T) {
	encryptedRaw := EncodeThinkingSignature("SIG")
	inputJSON := `[
		{"role":"user","content":"hi"},
		{"role":"assistant","content":[{"type":"output_text","text":"hello"}]},
		{"role":"developer","content":"think harder"},
		{"type":"reasoning","id":"rs_1","encrypted_content":"` + encryptedRaw + `","summary":[{"type":"summary_text","text":"reflect"}]}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(claude.Messages) != 4 {
		t.Fatalf("messages=%d want 4 (user, assistant-text, user-carrier, assistant-reasoning)", len(claude.Messages))
	}
	if claude.Messages[3].Role != "assistant" {
		t.Errorf("messages[3].role=%q want assistant", claude.Messages[3].Role)
	}
	last, _ := claude.Messages[3].ParseContent()
	if len(last) != 1 || last[0].Type != "thinking" {
		t.Errorf("last assistant should be thinking-only, got %+v", last)
	}
}

// Oracle round-2 gap: developer before a function_call_output (user with leading tool_result)
// must keep tool_result at the message head; wrapped text must end up after tool_result.
func TestDeveloperRoleBeforeFunctionCallOutput(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"call it"},
		{"type":"function_call","call_id":"call_1","name":"do","arguments":"{}"},
		{"role":"developer","content":"about to receive result"},
		{"type":"function_call_output","call_id":"call_1","output":"ok"}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(claude.Messages) != 3 {
		t.Fatalf("messages=%d want 3 (user, assistant tool_use, user with tool_result+wrapped)", len(claude.Messages))
	}
	if claude.Messages[2].Role != "user" {
		t.Fatalf("messages[2].role=%q want user", claude.Messages[2].Role)
	}
	blocks, _ := claude.Messages[2].ParseContent()
	if len(blocks) != 2 {
		t.Fatalf("messages[2] blocks=%d want 2 (tool_result + wrapped)", len(blocks))
	}
	if blocks[0].Type != "tool_result" {
		t.Errorf("blocks[0].type=%q want tool_result (must be at head)", blocks[0].Type)
	}
	if blocks[1].Type != "text" || blocks[1].Text == nil || !strings.Contains(*blocks[1].Text, "about to receive result") {
		t.Errorf("blocks[1] should be wrapped developer text, got %+v", blocks[1])
	}
}

// Oracle round-2 gap: two consecutive developer items in the middle should preserve order.
func TestDeveloperRoleTwoConsecutiveInterleavedPreservesOrder(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"first"},
		{"role":"developer","content":"rule A"},
		{"role":"developer","content":"rule B"},
		{"role":"user","content":"second"}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(claude.Messages) != 1 {
		t.Fatalf("messages=%d want 1 (both users merged with wrappers between)", len(claude.Messages))
	}
	blocks, _ := claude.Messages[0].ParseContent()
	if len(blocks) != 4 {
		t.Fatalf("blocks=%d want 4 (first, ruleA, ruleB, second)", len(blocks))
	}
	if *blocks[0].Text != "first" {
		t.Errorf("blocks[0]=%q want first", *blocks[0].Text)
	}
	if !strings.Contains(*blocks[1].Text, "rule A") {
		t.Errorf("blocks[1] should contain rule A, got %q", *blocks[1].Text)
	}
	if !strings.Contains(*blocks[2].Text, "rule B") {
		t.Errorf("blocks[2] should contain rule B, got %q", *blocks[2].Text)
	}
	if *blocks[3].Text != "second" {
		t.Errorf("blocks[3]=%q want second", *blocks[3].Text)
	}
}

// Oracle round-2 gap: instructions provided as an input_text array, combined with prefix dev.
func TestDeveloperRolePrefixWithInstructionsArray(t *testing.T) {
	instructionsRaw, _ := common.Marshal([]map[string]any{
		{"type": "input_text", "text": "base from instructions"},
	})
	inputJSON := `[
		{"role":"developer","content":"extra from input"},
		{"role":"user","content":"hi"}
	]`
	req := &dto.OpenAIResponsesRequest{
		Model:        "claude-opus-4-7",
		Input:        []byte(inputJSON),
		Instructions: instructionsRaw,
	}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	sysBlocks, ok := claude.System.([]dto.ClaudeMediaMessage)
	if !ok {
		t.Fatalf("System type=%T want []ClaudeMediaMessage", claude.System)
	}
	if len(sysBlocks) != 2 {
		t.Fatalf("system blocks=%d want 2", len(sysBlocks))
	}
	if *sysBlocks[0].Text != "base from instructions" || *sysBlocks[1].Text != "extra from input" {
		t.Errorf("system blocks wrong order: %q, %q", *sysBlocks[0].Text, *sysBlocks[1].Text)
	}
}

// Oracle round-3 gap: parallel tool_use with developer before the outputs.
// Two function_call_output items must keep tool_result blocks contiguous at head,
// wrapped developer text deferred to after the last tool_result.
func TestDeveloperRoleBeforeParallelFunctionCallOutputs(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"call both"},
		{"type":"function_call","call_id":"c1","name":"a","arguments":"{}"},
		{"type":"function_call","call_id":"c2","name":"b","arguments":"{}"},
		{"role":"developer","content":"about to receive results"},
		{"type":"function_call_output","call_id":"c1","output":"r1"},
		{"type":"function_call_output","call_id":"c2","output":"r2"}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(claude.Messages) != 3 {
		t.Fatalf("messages=%d want 3 (user, assistant(2 tool_use), user(2 tool_result + wrapped))", len(claude.Messages))
	}
	if claude.Messages[2].Role != "user" {
		t.Fatalf("messages[2].role=%q want user", claude.Messages[2].Role)
	}
	blocks, _ := claude.Messages[2].ParseContent()
	if len(blocks) != 3 {
		t.Fatalf("messages[2] blocks=%d want 3 ([tool_result, tool_result, wrapped]), got %+v", len(blocks), blocks)
	}
	if blocks[0].Type != "tool_result" || blocks[1].Type != "tool_result" {
		t.Errorf("first two blocks must be tool_result, got %q, %q", blocks[0].Type, blocks[1].Type)
	}
	if blocks[2].Type != "text" || blocks[2].Text == nil || !strings.Contains(*blocks[2].Text, "about to receive results") {
		t.Errorf("blocks[2] should be wrapped developer at tail, got %+v", blocks[2])
	}
}

// Oracle round-3 gap: lock in positive behavior when developer follows the tool_result run.
// function_call → output1 → output2 → developer → user normal
// developer should appear after both tool_results and before the next user query.
func TestDeveloperRoleAfterToolResultRunBeforeUser(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"call"},
		{"type":"function_call","call_id":"c1","name":"a","arguments":"{}"},
		{"type":"function_call_output","call_id":"c1","output":"r1"},
		{"role":"developer","content":"now switch"},
		{"role":"user","content":"next query"}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(claude.Messages) != 3 {
		t.Fatalf("messages=%d want 3 (user, assistant, user[tool_result, wrapped, next query])", len(claude.Messages))
	}
	blocks, _ := claude.Messages[2].ParseContent()
	if len(blocks) != 3 {
		t.Fatalf("user blocks=%d want 3, got %+v", len(blocks), blocks)
	}
	if blocks[0].Type != "tool_result" {
		t.Errorf("blocks[0] must be tool_result, got %q", blocks[0].Type)
	}
	if !strings.Contains(*blocks[1].Text, "now switch") {
		t.Errorf("blocks[1] should contain wrapped developer, got %q", *blocks[1].Text)
	}
	if *blocks[2].Text != "next query" {
		t.Errorf("blocks[2] should be next query, got %q", *blocks[2].Text)
	}
}

// Oracle round-3 gap: cannot insert developer between assistant tool_use and the matching tool_result.
// function_call → developer → assistant text (no tool_result) → reject.
func TestDeveloperRoleBetweenToolUseAndAssistantTextRejected(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"call"},
		{"type":"function_call","call_id":"c1","name":"a","arguments":"{}"},
		{"role":"developer","content":"sneaky"},
		{"role":"assistant","content":[{"type":"output_text","text":"answer"}]}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	_, _, _, err := ConvertResponsesRequestToClaude(req)
	if err == nil || !strings.Contains(err.Error(), "tool_use") {
		t.Errorf("expected rejection for developer between tool_use and non-tool_result, got %v", err)
	}
}

// Oracle round-3 gap: trailing developer right after assistant tool_use is also unrepresentable.
func TestDeveloperRoleTrailingAfterToolUseRejected(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"call"},
		{"type":"function_call","call_id":"c1","name":"a","arguments":"{}"},
		{"role":"developer","content":"trailing dev"}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	_, _, _, err := ConvertResponsesRequestToClaude(req)
	if err == nil || !strings.Contains(err.Error(), "tool_use") {
		t.Errorf("expected rejection for trailing developer after unresolved tool_use, got %v", err)
	}
}

// Oracle round-3 gap: developer between matched tool_use/tool_result and a later assistant_text
// is legal: the handshake is already closed by the tool_result, so wrapped can flow into the
// previous user(tool_result+wrapped) message.
func TestDeveloperRoleAfterClosedHandshakeBeforeAssistantText(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"call"},
		{"type":"function_call","call_id":"c1","name":"a","arguments":"{}"},
		{"type":"function_call_output","call_id":"c1","output":"r1"},
		{"role":"developer","content":"reflect"},
		{"role":"assistant","content":[{"type":"output_text","text":"final"}]}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	claude, _, _, err := ConvertResponsesRequestToClaude(req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(claude.Messages) != 4 {
		t.Fatalf("messages=%d want 4 (user, assistant tool_use, user[tool_result+wrapped], assistant final)", len(claude.Messages))
	}
	toolUser, _ := claude.Messages[2].ParseContent()
	if len(toolUser) != 2 || toolUser[0].Type != "tool_result" {
		t.Fatalf("user[tool_result+wrapped] wrong: %+v", toolUser)
	}
	if !strings.Contains(*toolUser[1].Text, "reflect") {
		t.Errorf("wrapped not present after tool_result, got %q", *toolUser[1].Text)
	}
}

// Oracle round-4 gap: developer between assistant tool_use and a normal user_text (no tool_result)
// is the same handshake violation as the assistant_text case; must also reject.
func TestDeveloperRoleBetweenToolUseAndUserTextRejected(t *testing.T) {
	inputJSON := `[
		{"role":"user","content":"call"},
		{"type":"function_call","call_id":"c1","name":"a","arguments":"{}"},
		{"role":"developer","content":"sneak between"},
		{"role":"user","content":"changed my mind"}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "claude-opus-4-7", Input: []byte(inputJSON)}
	_, _, _, err := ConvertResponsesRequestToClaude(req)
	if err == nil || !strings.Contains(err.Error(), "tool_use") {
		t.Errorf("expected rejection for developer between tool_use and normal user text, got %v", err)
	}
}
