package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDoImageResponse_whenEditReturnsSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := strings.Join([]string{
		`event: image_edit.partial_image`,
		`data: {"type":"image_edit.partial_image","b64_json":"partial"}`,
		``,
		`event: image_edit.completed`,
		`data: {"type":"image_edit.completed","b64_json":"final","usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7,"input_tokens_details":{"image_tokens":2,"text_tokens":1}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
		RelayMode:   relayconstant.RelayModeImagesEdits,
		IsStream:    true,
	}

	usage, relayErr := (&Adaptor{}).DoImageResponse(c, info, resp)
	require.Nil(t, relayErr)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 4, usage.CompletionTokens)
	require.Equal(t, 7, usage.TotalTokens)
	require.Equal(t, 2, usage.PromptTokensDetails.ImageTokens)
	require.Contains(t, recorder.Body.String(), "event: image_edit.partial_image")
	require.Contains(t, recorder.Body.String(), "event: image_edit.completed")
	require.Contains(t, recorder.Body.String(), "data: [DONE]")
}

func TestDoImageResponse_whenEditStreamFallsBackToJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"created":1710000000,"data":[{"b64_json":"final"}],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`,
		)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
		RelayMode:   relayconstant.RelayModeImagesEdits,
		IsStream:    true,
	}

	usage, relayErr := (&Adaptor{}).DoImageResponse(c, info, resp)
	require.Nil(t, relayErr)
	require.Equal(t, 3, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), "event: image_edit.completed")
	require.Contains(t, recorder.Body.String(), `"type":"image_edit.completed"`)
	require.Contains(t, recorder.Body.String(), "data: [DONE]")
}
