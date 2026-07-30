package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stagedResponseBody struct {
	reader  *strings.Reader
	release <-chan struct{}
}

func (b *stagedResponseBody) Read(p []byte) (int, error) {
	if b.reader.Len() > 0 {
		return b.reader.Read(p)
	}
	<-b.release
	return 0, io.EOF
}

func (b *stagedResponseBody) Close() error {
	return nil
}

type notifyingResponseWriter struct {
	gin.ResponseWriter
	wrote chan struct{}
	once  sync.Once
}

func (w *notifyingResponseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.once.Do(func() { close(w.wrote) })
	return n, err
}

func TestOaiResponsesStreamHandler_whenOverloadIsThirdEvent(t *testing.T) {
	// Given: 上游先发送两个生命周期事件，再报告服务过载。
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`,
		``,
		`event: response.in_progress`,
		`data: {"type":"response.in_progress","response":{"id":"resp_1","status":"in_progress"}}`,
		``,
		`event: error`,
		`data: {"type":"error","error":{"type":"service_unavailable_error","code":"server_is_overloaded","message":"Our servers are currently overloaded. Please try again later."}}`,
		``,
		`event: response.failed`,
		`data: {"type":"response.failed","response":{"id":"resp_1","status":"failed","error":{"code":"server_is_overloaded","message":"Our servers are currently overloaded. Please try again later."}}}`,
		``,
	}, "\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.6-sol"}}

	// When: 处理上游 SSE。
	_, relayErr := OaiResponsesStreamHandler(c, info, resp)

	// Then: 错误等待窗口内不能提交响应，控制器仍可重试。
	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusInternalServerError, relayErr.StatusCode)
	require.Equal(t, types.ErrorCode("server_is_overloaded"), relayErr.GetErrorCode())
	require.True(t, types.IsRecordErrorLog(relayErr))
	require.False(t, types.IsSkipRetryError(relayErr))
	require.Empty(t, recorder.Body.String())
}

func TestOaiResponsesStreamHandler_whenOverloadIsFirstEvent(t *testing.T) {
	// Given: 上游在任何可转发事件之前报告服务过载。
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`event: error`,
			`data: {"type":"error","error":{"type":"service_unavailable_error","code":"server_is_overloaded","message":"Our servers are currently overloaded. Please try again later."}}`,
			``,
		}, "\n"))),
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.6-sol"}}

	// When: 处理上游首个 SSE 错误。
	_, relayErr := OaiResponsesStreamHandler(c, info, resp)

	// Then: 尚未开始响应时仍允许控制器重试，且不发送错误事件。
	require.NotNil(t, relayErr)
	require.False(t, types.IsSkipRetryError(relayErr))
	require.Empty(t, recorder.Body.String())
}

func TestOaiResponsesStreamHandler_forwardsLifecycleEvents_whenErrorWindowExpires(t *testing.T) {
	// Given: 上游发送生命周期事件后保持连接，但没有继续输出或报错。
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	oldErrorWait := responsesStreamErrorWaitDuration
	responsesStreamErrorWaitDuration = 10 * time.Millisecond
	t.Cleanup(func() { responsesStreamErrorWaitDuration = oldErrorWait })

	releaseUpstream := make(chan struct{})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	wrote := make(chan struct{})
	c.Writer = &notifyingResponseWriter{ResponseWriter: c.Writer, wrote: wrote}
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: &stagedResponseBody{
			reader: strings.NewReader(strings.Join([]string{
				`event: response.created`,
				`data: {"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`,
				``,
			}, "\n")),
			release: releaseUpstream,
		},
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.6-sol"}}
	result := make(chan *types.NewAPIError, 1)
	go func() {
		_, relayErr := OaiResponsesStreamHandler(c, info, resp)
		result <- relayErr
	}()

	// When: 错误等待窗口到期。
	select {
	case <-wrote:
	case <-time.After(time.Second):
		t.Fatal("等待窗口到期后没有转发生命周期事件")
	}
	close(releaseUpstream)
	relayErr := <-result

	// Then: 生命周期事件已发送，流可以继续正常结束。
	require.Nil(t, relayErr)
	require.Contains(t, recorder.Body.String(), `"type":"response.created"`)
}
