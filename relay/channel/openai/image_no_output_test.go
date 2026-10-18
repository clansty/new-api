package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newImageTestResponse(contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newImageTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	return c, recorder
}

func newImageTestInfo(stream bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
		RelayMode:   relayconstant.RelayModeImagesGenerations,
		IsStream:    stream,
	}
}

func TestDoImageResponse_nonStreamEmptyDataIsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, recorder := newImageTestContext()

	usage, relayErr := (&Adaptor{}).DoImageResponse(c, newImageTestInfo(false),
		newImageTestResponse("application/json", `{"created":1710000000,"data":[]}`))

	require.Nil(t, usage)
	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
	require.False(t, types.IsSkipRetryError(relayErr))
	require.Empty(t, recorder.Body.String(), "失败响应不能写给客户端")
}

func TestDoImageResponse_nonStreamDataWithoutImageIsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newImageTestContext()

	_, relayErr := (&Adaptor{}).DoImageResponse(c, newImageTestInfo(false),
		newImageTestResponse("application/json", `{"created":1710000000,"data":[{"revised_prompt":"only text"}]}`))

	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
}

func TestDoImageResponse_nonStreamEmbeddedErrorIsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newImageTestContext()

	_, relayErr := (&Adaptor{}).DoImageResponse(c, newImageTestInfo(false),
		newImageTestResponse("application/json",
			`{"error":{"type":"image_generation_user_error","message":"moderation blocked"}}`))

	require.NotNil(t, relayErr)
	// 内容审核类拒绝是客户端错误，不能映射成可重试的 502
	require.Equal(t, http.StatusBadRequest, relayErr.StatusCode)
	require.Contains(t, relayErr.Error(), "moderation blocked")
}

func TestDoImageResponse_nonStreamWithImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, recorder := newImageTestContext()

	usage, relayErr := (&Adaptor{}).DoImageResponse(c, newImageTestInfo(false),
		newImageTestResponse("application/json",
			`{"created":1710000000,"data":[{"b64_json":"aGVsbG8="}],"usage":{"input_tokens":10,"output_tokens":20}}`))

	require.Nil(t, relayErr)
	require.NotNil(t, usage)
	require.Equal(t, 10, usage.PromptTokens)
	require.Equal(t, 20, usage.CompletionTokens)
	require.Contains(t, recorder.Body.String(), "aGVsbG8=")
}

func TestDoImageResponse_nonStreamResponsesStyleOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newImageTestContext()

	usage, relayErr := (&Adaptor{}).DoImageResponse(c, newImageTestInfo(false),
		newImageTestResponse("application/json",
			`{"output":[{"type":"image_generation_call","result":"aGVsbG8="}]}`))

	require.Nil(t, relayErr)
	require.NotNil(t, usage)
}

func TestDoImageResponse_streamCompletedWithoutImageIsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newImageTestContext()

	body := strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"r","status":"in_progress"}}`,
		``,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"id":"r","status":"completed","output":[]}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	_, relayErr := (&Adaptor{}).DoImageResponse(c, newImageTestInfo(true),
		newImageTestResponse("text/event-stream", body))

	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
	require.True(t, types.IsSkipRetryError(relayErr), "已转发事件后不能再重试")
}

func TestDoImageResponse_streamCompletedWithImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, recorder := newImageTestContext()

	body := strings.Join([]string{
		`event: image_generation.completed`,
		`data: {"type":"image_generation.completed","b64_json":"final","usage":{"input_tokens":3,"output_tokens":4}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	usage, relayErr := (&Adaptor{}).DoImageResponse(c, newImageTestInfo(true),
		newImageTestResponse("text/event-stream", body))

	require.Nil(t, relayErr)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 4, usage.CompletionTokens)
	require.Contains(t, recorder.Body.String(), `"b64_json":"final"`)
}

func TestDoImageResponse_streamResponsesStyleUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newImageTestContext()

	body := strings.Join([]string{
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"status":"completed","output":[{"type":"image_generation_call","result":"final"}],"usage":{"input_tokens":12,"output_tokens":34}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	usage, relayErr := (&Adaptor{}).DoImageResponse(c, newImageTestInfo(true),
		newImageTestResponse("text/event-stream", body))

	require.Nil(t, relayErr)
	require.Equal(t, 12, usage.PromptTokens)
	require.Equal(t, 34, usage.CompletionTokens)
}

func TestDoImageResponse_streamPartialImageOnlyIsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newImageTestContext()

	body := strings.Join([]string{
		`event: image_generation.partial_image`,
		`data: {"type":"image_generation.partial_image","b64_json":"partial"}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	_, relayErr := (&Adaptor{}).DoImageResponse(c, newImageTestInfo(true),
		newImageTestResponse("text/event-stream", body))

	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
}

func TestDoImageResponse_jsonAsStreamEmptyDataIsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newImageTestContext()

	_, relayErr := (&Adaptor{}).DoImageResponse(c, newImageTestInfo(true),
		newImageTestResponse("application/json", `{"created":1710000000,"data":[]}`))

	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
}

func TestDoImageResponse_streamNullErrorFieldIsNotFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newImageTestContext()

	body := strings.Join([]string{
		`event: image_generation.completed`,
		`data: {"type":"image_generation.completed","b64_json":"final","error":null}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	_, relayErr := (&Adaptor{}).DoImageResponse(c, newImageTestInfo(true),
		newImageTestResponse("text/event-stream", body))

	require.Nil(t, relayErr)
}

func TestDoImageResponse_streamResponseFailedIsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newImageTestContext()

	body := strings.Join([]string{
		`event: response.failed`,
		`data: {"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","message":"boom"}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	_, relayErr := (&Adaptor{}).DoImageResponse(c, newImageTestInfo(true),
		newImageTestResponse("text/event-stream", body))

	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
}
