package common

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/require"
)

func TestRelayInfo_ShouldUseChannelResponseTimeout_when_request_is_normal_text(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		info *RelayInfo
	}{
		{
			name: "chat completions",
			info: &RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions, Request: &dto.GeneralOpenAIRequest{}},
		},
		{
			name: "chat completions with search disabled",
			info: &RelayInfo{
				RelayMode: relayconstant.RelayModeChatCompletions,
				Request:   &dto.GeneralOpenAIRequest{EnableSearch: []byte("false")},
			},
		},
		{
			name: "responses",
			info: &RelayInfo{RelayMode: relayconstant.RelayModeResponses, Request: &dto.OpenAIResponsesRequest{}},
		},
		{
			name: "messages",
			info: &RelayInfo{RelayMode: relayconstant.RelayModeClaudeMessages, Request: &dto.ClaudeRequest{}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.True(t, test.info.ShouldUseChannelResponseTimeout())
		})
	}
}

func TestRelayInfo_ShouldUseChannelResponseTimeout_when_request_is_excluded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		info *RelayInfo
	}{
		{
			name: "responses compaction",
			info: &RelayInfo{RelayMode: relayconstant.RelayModeResponsesCompact},
		},
		{
			name: "image generation",
			info: &RelayInfo{RelayMode: relayconstant.RelayModeImagesGenerations},
		},
		{
			name: "standalone web search",
			info: &RelayInfo{RelayMode: relayconstant.RelayModeAlphaSearch},
		},
		{
			name: "channel test",
			info: &RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions, IsChannelTest: true},
		},
		{
			name: "responses built-in web search",
			info: &RelayInfo{
				RelayMode: relayconstant.RelayModeResponses,
				ResponsesUsageInfo: &ResponsesUsageInfo{BuiltInTools: map[string]*BuildInToolInfo{
					dto.BuildInToolWebSearch: {ToolName: dto.BuildInToolWebSearch},
				}},
			},
		},
		{
			name: "chat web search",
			info: &RelayInfo{
				RelayMode: relayconstant.RelayModeChatCompletions,
				Request:   &dto.GeneralOpenAIRequest{WebSearchOptions: &dto.WebSearchOptions{}},
			},
		},
		{
			name: "chat built-in web search tool",
			info: &RelayInfo{
				RelayMode: relayconstant.RelayModeChatCompletions,
				Request: &dto.GeneralOpenAIRequest{Tools: []dto.ToolCallRequest{
					{Type: dto.BuildInToolWebSearch},
				}},
			},
		},
		{
			name: "messages web search",
			info: &RelayInfo{
				RelayMode: relayconstant.RelayModeClaudeMessages,
				Request: &dto.ClaudeRequest{Tools: []any{
					map[string]any{"type": "web_search_20250305", "name": "web_search"},
				}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.False(t, test.info.ShouldUseChannelResponseTimeout())
		})
	}
}

func TestChannelResponseTimeout_CancelsContext_when_no_response_arrives(t *testing.T) {
	t.Parallel()

	timeout := NewChannelResponseTimeout(context.Background(), 10*time.Millisecond)
	t.Cleanup(timeout.Stop)

	select {
	case <-timeout.Context().Done():
		require.True(t, timeout.TimedOut())
	case <-time.After(time.Second):
		t.Fatal("等待渠道响应超时取消上下文失败")
	}
}

func TestChannelResponseTimeout_StopsTimer_when_first_response_arrives(t *testing.T) {
	t.Parallel()

	timeout := NewChannelResponseTimeout(context.Background(), 20*time.Millisecond)
	t.Cleanup(timeout.Stop)
	timeout.MarkResponseStarted()

	select {
	case <-timeout.Context().Done():
		t.Fatal("收到首个响应后不应再触发渠道超时")
	case <-time.After(60 * time.Millisecond):
		require.False(t, timeout.TimedOut())
	}
}

func TestChannelResponseTimeout_WrapBody_marks_first_read_as_response(t *testing.T) {
	t.Parallel()

	timeout := NewChannelResponseTimeout(context.Background(), 20*time.Millisecond)
	t.Cleanup(timeout.Stop)
	body := timeout.WrapBody(io.NopCloser(strings.NewReader("ok")))

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, "ok", string(data))

	select {
	case <-timeout.Context().Done():
		t.Fatal("读取到首字后不应再触发渠道超时")
	case <-time.After(60 * time.Millisecond):
		require.False(t, timeout.TimedOut())
	}
}
