package channel

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

var initChannelResponseTimeoutHTTPClient sync.Once

func TestDoRequest_ChannelResponseTimeout_whenHeadersArriveWithoutBody(t *testing.T) {
	t.Parallel()
	initChannelResponseTimeoutHTTPClient.Do(service.InitHttpClient)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	timeout := relaycommon.NewChannelResponseTimeout(t.Context(), 20*time.Millisecond)
	t.Cleanup(timeout.Stop)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
	c.Request = c.Request.WithContext(timeout.Context())
	req, err := http.NewRequestWithContext(timeout.Context(), http.MethodPost, server.URL, strings.NewReader("{}"))
	require.NoError(t, err)

	resp, err := DoRequest(c, req, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
	require.NoError(t, err)
	require.NotNil(t, resp)
	t.Cleanup(func() { _ = resp.Body.Close() })

	_, err = io.ReadAll(resp.Body)
	require.Error(t, err)
	require.True(t, timeout.TimedOut())
}

func TestDoRequest_ChannelResponseTimeout_stopsAfterFirstBodyByte(t *testing.T) {
	t.Parallel()
	initChannelResponseTimeoutHTTPClient.Do(service.InitHttpClient)

	releaseResponse := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("a"))
		w.(http.Flusher).Flush()
		<-releaseResponse
		_, _ = w.Write([]byte("b"))
	}))
	t.Cleanup(server.Close)

	timeout := relaycommon.NewChannelResponseTimeout(t.Context(), 20*time.Millisecond)
	t.Cleanup(timeout.Stop)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
	c.Request = c.Request.WithContext(timeout.Context())
	req, err := http.NewRequestWithContext(timeout.Context(), http.MethodPost, server.URL, strings.NewReader("{}"))
	require.NoError(t, err)

	resp, err := DoRequest(c, req, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
	require.NoError(t, err)
	require.NotNil(t, resp)
	t.Cleanup(func() { _ = resp.Body.Close() })

	firstByte := make([]byte, 1)
	read, err := resp.Body.Read(firstByte)
	require.NoError(t, err)
	require.Equal(t, 1, read)
	require.Equal(t, "a", string(firstByte))

	select {
	case <-timeout.Context().Done():
		t.Fatal("读取到首字后渠道超时不应再取消请求")
	case <-time.After(60 * time.Millisecond):
	}

	close(releaseResponse)
	rest, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "b", string(rest))
	require.False(t, timeout.TimedOut())
}
