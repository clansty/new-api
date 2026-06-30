package claude

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 固定的服务器时间，用于使日期相关断言确定
var fixedNow = time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)

func TestCleanText_ApostropheVariants(t *testing.T) {
	cases := []struct {
		name    string
		variant rune
		label   string
	}{
		{"right_single_quote", '\u2019', "U+2019"},
		{"modifier_apostrophe", '\u02BC', "U+02BC"},
		{"modifier_prime", '\u02B9', "U+02B9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := "You are Claude. Today" + string(tc.variant) + "s date is 2026-06-30."
			cleaned, details := cleanText(input, fixedNow, true)

			require.Equal(t, "You are Claude. Today's date is 2026-06-30.", cleaned)
			require.Len(t, details, 1)
			require.Equal(t, dto.SystemPromptCleanApostrophe, details[0].Type)
			require.Equal(t, tc.label, details[0].From)
			require.Equal(t, "U+0027", details[0].To)
		})
	}
}

func TestCleanText_DateFormatViaPhrase(t *testing.T) {
	cases := []struct {
		name  string
		slash string
		dash  string
	}{
		{"yesterday", "2026/06/29", "2026-06-29"},
		{"today", "2026/06/30", "2026-06-30"},
		{"tomorrow", "2026/07/01", "2026-07-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := "Today's date is " + tc.slash + "."
			cleaned, details := cleanText(input, fixedNow, true)

			require.Equal(t, "Today's date is "+tc.dash+".", cleaned)
			require.Len(t, details, 1)
			require.Equal(t, dto.SystemPromptCleanDateFormat, details[0].Type)
			require.Equal(t, tc.slash, details[0].From)
			require.Equal(t, tc.dash, details[0].To)
		})
	}
}

// 关键回归：短语锚定的日期清洗与服务器时钟无关——即使服务器日期远不等于提示词里的日期也应清洗
func TestCleanText_PhraseAnchoredClockIndependent(t *testing.T) {
	farPast := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	input := "<env>\nToday's date is 2026/06/30\n</env>"
	cleaned, details := cleanText(input, farPast, false)

	require.Equal(t, "<env>\nToday's date is 2026-06-30\n</env>", cleaned)
	require.Len(t, details, 1)
	require.Equal(t, dto.SystemPromptCleanDateFormat, details[0].Type)
	require.Equal(t, "2026/06/30", details[0].From)
	require.Equal(t, "2026-06-30", details[0].To)
}

func TestCleanText_DateColonVariantAndApostropheVariant(t *testing.T) {
	input := "<env>\nToday\u2019s date: 2026/06/30\n</env>"
	cleaned, details := cleanText(input, fixedNow, false)

	require.Equal(t, "<env>\nToday's date: 2026-06-30\n</env>", cleaned)
	require.Len(t, details, 2)
	require.Equal(t, dto.SystemPromptCleanApostrophe, details[0].Type)
	require.Equal(t, dto.SystemPromptCleanDateFormat, details[1].Type)
}

// 无短语时，仅在 useDateHeuristic=true 且日期落在服务器日期 ±1 才替换
func TestCleanText_HeuristicScopedToSystemOnly(t *testing.T) {
	bare := "raw date 2026/06/30 without phrase"

	withHeuristic, dH := cleanText(bare, fixedNow, true)
	require.Equal(t, "raw date 2026-06-30 without phrase", withHeuristic)
	require.Len(t, dH, 1)

	noHeuristic, dN := cleanText(bare, fixedNow, false)
	require.Equal(t, bare, noHeuristic)
	require.Empty(t, dN)
}

func TestCleanText_DateOutsideRangeUntouched(t *testing.T) {
	input := "Some unrelated date 2020/01/01 and two days ahead 2026/07/02."
	cleaned, details := cleanText(input, fixedNow, true)

	require.Equal(t, input, cleaned)
	require.Empty(t, details)
}

func TestCleanText_NoOp(t *testing.T) {
	clean := "Today's date is 2026-06-30. Nothing to fix here."
	cleaned, details := cleanText(clean, fixedNow, true)
	require.Equal(t, clean, cleaned)
	require.Empty(t, details)

	cleanedEmpty, detailsEmpty := cleanText("", fixedNow, true)
	require.Equal(t, "", cleanedEmpty)
	require.Empty(t, detailsEmpty)
}

func TestCleanClaudeRequest_StringSystem(t *testing.T) {
	req := &dto.ClaudeRequest{
		Model:  "claude-opus-4-8",
		System: "Today\u02BCs date is " + slashToday() + ".",
	}
	details := CleanClaudeRequest(req)

	require.True(t, req.IsStringSystem())
	require.Equal(t, "Today's date is "+dashToday()+".", req.GetStringSystem())
	require.Len(t, details, 2)
}

func TestCleanClaudeRequest_ArraySystem(t *testing.T) {
	corrupted := "Today\u2019s date is " + slashToday() + "."
	req := &dto.ClaudeRequest{
		Model: "claude-opus-4-8",
		System: []dto.ClaudeMediaMessage{
			{Type: dto.ContentTypeText, Text: strPtr(corrupted)},
			{Type: dto.ContentTypeText, Text: strPtr("clean block")},
		},
	}
	details := CleanClaudeRequest(req)

	require.Len(t, details, 2)
	contents := req.ParseSystem()
	require.Equal(t, "Today's date is "+dashToday()+".", contents[0].GetText())
	require.Equal(t, "clean block", contents[1].GetText())
}

// 指纹在第一条用户消息（Claude Code 的 <env> 块）时也应被清洗
func TestCleanClaudeRequest_CleansFirstUserMessage(t *testing.T) {
	req := &dto.ClaudeRequest{
		Model:  "claude-opus-4-8",
		System: "You are Claude Code.",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "<env>\nToday's date is 2026/06/30\n</env>"},
			}},
		},
	}
	details := CleanClaudeRequest(req)

	require.Len(t, details, 1)
	require.Equal(t, dto.SystemPromptCleanDateFormat, details[0].Type)
	blocks, err := req.Messages[0].ParseContent()
	require.NoError(t, err)
	require.Equal(t, "<env>\nToday's date is 2026-06-30\n</env>", blocks[0].GetText())
}

// 只清理第一条用户消息，后续真实用户输入不应被改动
func TestCleanClaudeRequest_OnlyFirstUserMessage(t *testing.T) {
	req := &dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "Today's date is 2026/06/30"},
			{Role: "assistant", Content: "ok"},
			{Role: "user", Content: "Today's date is 2026/06/30"},
		},
	}
	details := CleanClaudeRequest(req)

	require.Len(t, details, 1)
	require.Equal(t, "Today's date is 2026-06-30", req.Messages[0].Content)
	require.Equal(t, "Today's date is 2026/06/30", req.Messages[2].Content)
}

func TestCleanClaudeRequest_NilAndNoSystem(t *testing.T) {
	require.Nil(t, CleanClaudeRequest(nil))
	require.Nil(t, CleanClaudeRequest(&dto.ClaudeRequest{Model: "claude-opus-4-8"}))
}

func TestCleanClaudeRequestBody_StringSystemPreservesOtherFields(t *testing.T) {
	body := fmt.Sprintf(`{"model":"claude-opus-4-8","max_tokens":1024,"system":"Today\u02b9s date is %s."}`, slashToday())
	cleaned, details := CleanClaudeRequestBody([]byte(body))

	require.Len(t, details, 2)
	require.Equal(t, "Today's date is "+dashToday()+".", gjson.GetBytes(cleaned, "system").String())
	require.Equal(t, "claude-opus-4-8", gjson.GetBytes(cleaned, "model").String())
	require.Equal(t, int64(1024), gjson.GetBytes(cleaned, "max_tokens").Int())
}

func TestCleanClaudeRequestBody_ArraySystem(t *testing.T) {
	body := fmt.Sprintf(`{"model":"claude-opus-4-8","system":[{"type":"text","text":"Today\u2019s date is %s."},{"type":"text","text":"unchanged"}]}`, slashToday())
	cleaned, details := CleanClaudeRequestBody([]byte(body))

	require.Len(t, details, 2)
	require.Equal(t, "Today's date is "+dashToday()+".", gjson.GetBytes(cleaned, "system.0.text").String())
	require.Equal(t, "unchanged", gjson.GetBytes(cleaned, "system.1.text").String())
}

// 直通字节路径：清洗第一条用户消息的内容块（Claude Code 真实场景）
func TestCleanClaudeRequestBody_CleansFirstUserMessage(t *testing.T) {
	body := `{"model":"claude-opus-4-8","system":"You are Claude Code.","messages":[{"role":"user","content":[{"type":"text","text":"<env>\nToday's date is 2026/06/30\n</env>"}]}]}`
	cleaned, details := CleanClaudeRequestBody([]byte(body))

	require.Len(t, details, 1)
	require.Equal(t, dto.SystemPromptCleanDateFormat, details[0].Type)
	require.Equal(t, "<env>\nToday's date is 2026-06-30\n</env>", gjson.GetBytes(cleaned, "messages.0.content.0.text").String())
	require.Equal(t, "You are Claude Code.", gjson.GetBytes(cleaned, "system").String())
}

func TestCleanClaudeRequestBody_StringMessageContent(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"Today\u2019s date is 2026/06/30"}]}`
	cleaned, details := CleanClaudeRequestBody([]byte(body))

	require.Len(t, details, 2)
	require.Equal(t, "Today's date is 2026-06-30", gjson.GetBytes(cleaned, "messages.0.content").String())
}

func TestCleanClaudeRequestBody_NoSystemOrNoOp(t *testing.T) {
	noSystem := []byte(`{"model":"claude-opus-4-8"}`)
	cleaned, details := CleanClaudeRequestBody(noSystem)
	require.Empty(t, details)
	require.Equal(t, noSystem, cleaned)

	cleanBody := []byte(`{"model":"claude-opus-4-8","system":"Today's date is 2026-06-30."}`)
	cleaned2, details2 := CleanClaudeRequestBody(cleanBody)
	require.Empty(t, details2)
	require.Equal(t, cleanBody, cleaned2)
}

func slashToday() string { return time.Now().Format("2006/01/02") }
func dashToday() string  { return time.Now().Format("2006-01-02") }
func strPtr(s string) *string {
	return &s
}
