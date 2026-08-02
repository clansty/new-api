package common

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/dto"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

const (
	channelTimeoutPending int32 = iota
	channelTimeoutResponded
	channelTimeoutExpired
	channelTimeoutStopped
)

var excludedResponsesToolTypes = map[string]struct{}{
	"code_interpreter":     {},
	"computer":             {},
	"computer_use_preview": {},
	"file_search":          {},
	"image_generation":     {},
	"mcp":                  {},
	"web_search":           {},
	"web_search_preview":   {},
}

type channelResponseTimeoutContextKey struct{}

type ChannelResponseTimeoutError struct {
	Timeout time.Duration
}

func (e *ChannelResponseTimeoutError) Error() string {
	return fmt.Sprintf("渠道响应超时：%s 内未收到上游响应", e.Timeout)
}

type ChannelResponseTimeout struct {
	ctx    context.Context
	cancel context.CancelFunc
	timer  *time.Timer
	state  atomic.Int32
}

func NewChannelResponseTimeout(parent context.Context, timeout time.Duration) *ChannelResponseTimeout {
	ctx, cancel := context.WithCancel(parent)
	result := &ChannelResponseTimeout{cancel: cancel}
	result.ctx = context.WithValue(ctx, channelResponseTimeoutContextKey{}, result)
	result.timer = time.AfterFunc(timeout, func() {
		if result.state.CompareAndSwap(channelTimeoutPending, channelTimeoutExpired) {
			cancel()
		}
	})
	return result
}

func (t *ChannelResponseTimeout) Context() context.Context {
	return t.ctx
}

func (t *ChannelResponseTimeout) TimedOut() bool {
	return t != nil && t.state.Load() == channelTimeoutExpired
}

func (t *ChannelResponseTimeout) MarkResponseStarted() {
	if t == nil || !t.state.CompareAndSwap(channelTimeoutPending, channelTimeoutResponded) {
		return
	}
	t.timer.Stop()
}

func (t *ChannelResponseTimeout) Stop() {
	if t == nil {
		return
	}
	if t.state.CompareAndSwap(channelTimeoutPending, channelTimeoutStopped) {
		t.timer.Stop()
	}
	t.cancel()
}

func (t *ChannelResponseTimeout) WrapBody(body io.ReadCloser) io.ReadCloser {
	if t == nil || body == nil {
		return body
	}
	return &channelResponseBody{
		ReadCloser: body,
		timeout:    t,
	}
}

type channelResponseBody struct {
	io.ReadCloser
	timeout *ChannelResponseTimeout
	once    sync.Once
}

func (b *channelResponseBody) Read(data []byte) (int, error) {
	read, err := b.ReadCloser.Read(data)
	if read > 0 || err != nil {
		b.once.Do(b.timeout.MarkResponseStarted)
	}
	return read, err
}

func GetChannelResponseTimeout(ctx context.Context) *ChannelResponseTimeout {
	timeout, _ := ctx.Value(channelResponseTimeoutContextKey{}).(*ChannelResponseTimeout)
	return timeout
}

func WrapChannelResponseBody(ctx context.Context, body io.ReadCloser) io.ReadCloser {
	return GetChannelResponseTimeout(ctx).WrapBody(body)
}

func MarkChannelResponseStarted(ctx context.Context) {
	GetChannelResponseTimeout(ctx).MarkResponseStarted()
}

func (info *RelayInfo) ShouldUseChannelResponseTimeout() bool {
	if info == nil || info.IsChannelTest {
		return false
	}
	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions:
		return !isChatSearchRequest(info.Request)
	case relayconstant.RelayModeResponses:
		return !info.hasExcludedResponsesTool()
	case relayconstant.RelayModeClaudeMessages:
		return !isClaudeSearchRequest(info.Request)
	default:
		return false
	}
}

func (info *RelayInfo) hasExcludedResponsesTool() bool {
	if info.ResponsesUsageInfo == nil {
		return false
	}
	for toolType := range info.ResponsesUsageInfo.BuiltInTools {
		if _, excluded := excludedResponsesToolTypes[strings.ToLower(toolType)]; excluded {
			return true
		}
	}
	return false
}

func isChatSearchRequest(request dto.Request) bool {
	chatRequest, ok := request.(*dto.GeneralOpenAIRequest)
	if !ok || chatRequest == nil {
		return false
	}
	for _, tool := range chatRequest.Tools {
		if _, excluded := excludedResponsesToolTypes[strings.ToLower(tool.Type)]; excluded {
			return true
		}
	}
	return chatRequest.WebSearchOptions != nil ||
		rawSearchOptionEnabled(chatRequest.SearchParameters) ||
		rawSearchOptionEnabled(chatRequest.EnableSearch) ||
		rawSearchOptionEnabled(chatRequest.WebSearch) ||
		rawSearchOptionEnabled(chatRequest.SearchDomainFilter) ||
		rawSearchOptionEnabled(chatRequest.SearchRecencyFilter) ||
		rawSearchOptionEnabled(chatRequest.SearchMode)
}

func rawSearchOptionEnabled(value []byte) bool {
	switch strings.ToLower(strings.TrimSpace(string(value))) {
	case "", "false", "null", "0", `""`, "[]", "{}":
		return false
	default:
		return true
	}
}

func isClaudeSearchRequest(request dto.Request) bool {
	claudeRequest, ok := request.(*dto.ClaudeRequest)
	if !ok || claudeRequest == nil {
		return false
	}
	for _, tool := range claudeRequest.GetTools() {
		switch value := tool.(type) {
		case *dto.ClaudeWebSearchTool:
			return true
		case dto.ClaudeWebSearchTool:
			return true
		case map[string]any:
			toolType, _ := value["type"].(string)
			toolName, _ := value["name"].(string)
			if strings.HasPrefix(strings.ToLower(toolType), "web_search") || strings.EqualFold(toolName, "web_search") {
				return true
			}
		}
	}
	return false
}
