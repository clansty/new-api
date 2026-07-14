package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertJSONImageRequest_whenEditingByURL(t *testing.T) {
	request := dto.ImageRequest{
		Model:         "gpt-image-1",
		Prompt:        "replace the background",
		Images:        json.RawMessage(`[{"image_url":"https://example.com/source.png"}]`),
		Mask:          json.RawMessage(`{"file_id":"file-mask"}`),
		InputFidelity: json.RawMessage(`"high"`),
		Stream:        common.GetPointer(false),
	}

	converted := (&Adaptor{}).ConvertJSONImageRequest(request)
	encoded, err := common.Marshal(converted)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model":"gpt-image-1",
		"prompt":"replace the background",
		"images":[{"image_url":"https://example.com/source.png"}],
		"mask":{"file_id":"file-mask"},
		"input_fidelity":"high",
		"stream":false
	}`, string(encoded))
}

func TestConvertImageRequest_whenMultipartEditHasMultipleImages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1"))
	require.NoError(t, writer.WriteField("prompt", "combine the products"))
	require.NoError(t, writer.WriteField("input_fidelity", "high"))
	for _, name := range []string{"first.png", "second.png"} {
		part, err := writer.CreateFormFile("image[]", name)
		require.NoError(t, err)
		_, err = part.Write([]byte(name))
		require.NoError(t, err)
	}
	maskPart, err := writer.CreateFormFile("mask", "mask.png")
	require.NoError(t, err)
	_, err = maskPart.Write([]byte("mask"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, c.Request.ParseMultipartForm(32<<20))

	converted, err := (&Adaptor{}).ConvertImageRequest(c, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
	}, dto.ImageRequest{Model: "gpt-image-1"})
	require.NoError(t, err)

	convertedBody, ok := converted.(*bytes.Buffer)
	require.True(t, ok)
	replayed := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(convertedBody.Bytes()))
	replayed.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	require.NoError(t, replayed.ParseMultipartForm(32<<20))
	require.Equal(t, "combine the products", replayed.PostForm.Get("prompt"))
	require.Equal(t, "high", replayed.PostForm.Get("input_fidelity"))
	require.Len(t, replayed.MultipartForm.File["image[]"], 2)
	require.Len(t, replayed.MultipartForm.File["mask"], 1)

	first, err := replayed.MultipartForm.File["image[]"][0].Open()
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.Close() })
	content, err := io.ReadAll(first)
	require.NoError(t, err)
	require.Equal(t, []byte("first.png"), content)
}
