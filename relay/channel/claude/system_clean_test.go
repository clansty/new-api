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

func TestCleanSystemText_ApostropheVariants(t *testing.T) {
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
			cleaned, details := cleanSystemText(input, fixedNow)

			require.Equal(t, "You are Claude. Today's date is 2026-06-30.", cleaned)
			require.Len(t, details, 1)
			require.Equal(t, dto.SystemPromptCleanApostrophe, details[0].Type)
			require.Equal(t, tc.label, details[0].From)
			require.Equal(t, "U+0027", details[0].To)
		})
	}
}

func TestCleanSystemText_DateFormatWithinRange(t *testing.T) {
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
			cleaned, details := cleanSystemText(input, fixedNow)

			require.Equal(t, "Today's date is "+tc.dash+".", cleaned)
			require.Len(t, details, 1)
			require.Equal(t, dto.SystemPromptCleanDateFormat, details[0].Type)
			require.Equal(t, tc.slash, details[0].From)
			require.Equal(t, tc.dash, details[0].To)
		})
	}
}

func TestCleanSystemText_DateOutsideRangeUntouched(t *testing.T) {
	input := "Some unrelated date 2020/01/01 and two days ahead 2026/07/02."
	cleaned, details := cleanSystemText(input, fixedNow)

	require.Equal(t, input, cleaned)
	require.Empty(t, details)
}

func TestCleanSystemText_ApostropheAndDateCombined(t *testing.T) {
	input := "Today\u2019s date is 2026/06/30."
	cleaned, details := cleanSystemText(input, fixedNow)

	require.Equal(t, "Today's date is 2026-06-30.", cleaned)
	require.Len(t, details, 2)
	require.Equal(t, dto.SystemPromptCleanApostrophe, details[0].Type)
	require.Equal(t, dto.SystemPromptCleanDateFormat, details[1].Type)
}

func TestCleanSystemText_NoOp(t *testing.T) {
	clean := "Today's date is 2026-06-30. Nothing to fix here."
	cleaned, details := cleanSystemText(clean, fixedNow)
	require.Equal(t, clean, cleaned)
	require.Empty(t, details)

	cleanedEmpty, detailsEmpty := cleanSystemText("", fixedNow)
	require.Equal(t, "", cleanedEmpty)
	require.Empty(t, detailsEmpty)
}

func TestCleanClaudeSystemPrompt_StringSystem(t *testing.T) {
	req := &dto.ClaudeRequest{
		Model:  "claude-opus-4-8",
		System: "Today\u02BCs date is " + slashToday() + ".",
	}
	details := CleanClaudeSystemPrompt(req)

	require.True(t, req.IsStringSystem())
	require.Equal(t, "Today's date is "+dashToday()+".", req.GetStringSystem())
	require.Len(t, details, 2)
}

func TestCleanClaudeSystemPrompt_ArraySystem(t *testing.T) {
	corrupted := "Today\u2019s date is " + slashToday() + "."
	req := &dto.ClaudeRequest{
		Model: "claude-opus-4-8",
		System: []dto.ClaudeMediaMessage{
			{Type: dto.ContentTypeText, Text: strPtr(corrupted)},
			{Type: dto.ContentTypeText, Text: strPtr("clean block")},
		},
	}
	details := CleanClaudeSystemPrompt(req)

	require.Len(t, details, 2)
	contents := req.ParseSystem()
	require.Equal(t, "Today's date is "+dashToday()+".", contents[0].GetText())
	require.Equal(t, "clean block", contents[1].GetText())
}

func TestCleanClaudeSystemPrompt_NilAndNoSystem(t *testing.T) {
	require.Nil(t, CleanClaudeSystemPrompt(nil))
	require.Nil(t, CleanClaudeSystemPrompt(&dto.ClaudeRequest{Model: "claude-opus-4-8"}))
}

func TestCleanClaudeSystemPromptBody_StringSystemPreservesOtherFields(t *testing.T) {
	body := fmt.Sprintf(`{"model":"claude-opus-4-8","max_tokens":1024,"system":"Today\u02b9s date is %s."}`, slashToday())
	cleaned, details := CleanClaudeSystemPromptBody([]byte(body))

	require.Len(t, details, 2)
	require.Equal(t, "Today's date is "+dashToday()+".", gjson.GetBytes(cleaned, "system").String())
	// 其余字段必须原样保留
	require.Equal(t, "claude-opus-4-8", gjson.GetBytes(cleaned, "model").String())
	require.Equal(t, int64(1024), gjson.GetBytes(cleaned, "max_tokens").Int())
}

func TestCleanClaudeSystemPromptBody_ArraySystem(t *testing.T) {
	body := fmt.Sprintf(`{"model":"claude-opus-4-8","system":[{"type":"text","text":"Today\u2019s date is %s."},{"type":"text","text":"unchanged"}]}`, slashToday())
	cleaned, details := CleanClaudeSystemPromptBody([]byte(body))

	require.Len(t, details, 2)
	require.Equal(t, "Today's date is "+dashToday()+".", gjson.GetBytes(cleaned, "system.0.text").String())
	require.Equal(t, "unchanged", gjson.GetBytes(cleaned, "system.1.text").String())
}

func TestCleanClaudeSystemPromptBody_NoSystemOrNoOp(t *testing.T) {
	noSystem := []byte(`{"model":"claude-opus-4-8"}`)
	cleaned, details := CleanClaudeSystemPromptBody(noSystem)
	require.Empty(t, details)
	require.Equal(t, noSystem, cleaned)

	cleanBody := []byte(`{"model":"claude-opus-4-8","system":"Today's date is 2026-06-30."}`)
	cleaned2, details2 := CleanClaudeSystemPromptBody(cleanBody)
	require.Empty(t, details2)
	require.Equal(t, cleanBody, cleaned2)
}

func slashToday() string { return time.Now().Format("2006/01/02") }
func dashToday() string  { return time.Now().Format("2006-01-02") }
func strPtr(s string) *string {
	return &s
}
