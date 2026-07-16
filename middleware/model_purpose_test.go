package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	appI18n "github.com/QuantumNous/new-api/i18n"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestModelPurpose_whenRelayModeVaries(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		relayMode int
		want      ratio_setting.ModelPurpose
	}{
		{name: "聊天补全", path: "/v1/chat/completions", relayMode: relayconstant.RelayModeChatCompletions, want: ratio_setting.ModelPurposeChat},
		{name: "图片生成", path: "/v1/images/generations", relayMode: relayconstant.RelayModeImagesGenerations, want: ratio_setting.ModelPurposeImage},
		{name: "向量嵌入", path: "/v1/embeddings", relayMode: relayconstant.RelayModeEmbeddings, want: ratio_setting.ModelPurposeEmbedding},
		{name: "Gemini 向量嵌入", path: "/v1beta/models/text-embedding-004:embedContent", relayMode: relayconstant.RelayModeGemini, want: ratio_setting.ModelPurposeEmbedding},
		{name: "音频转写", path: "/v1/audio/transcriptions", relayMode: relayconstant.RelayModeAudioTranscription, want: ratio_setting.ModelPurposeAudio},
		{name: "重排", path: "/v1/rerank", relayMode: relayconstant.RelayModeRerank, want: ratio_setting.ModelPurposeRerank},
		{name: "内容审核", path: "/v1/moderations", relayMode: relayconstant.RelayModeModerations, want: ratio_setting.ModelPurposeModeration},
		{name: "视频生成", path: "/v1/videos", relayMode: relayconstant.RelayModeVideoSubmit, want: ratio_setting.ModelPurposeVideo},
		{name: "音乐生成", path: "/suno/submit/music", relayMode: relayconstant.RelayModeSunoSubmit, want: ratio_setting.ModelPurposeMusic},
		{name: "Midjourney 生图", path: "/mj/submit/imagine", relayMode: relayconstant.RelayModeMidjourneyImagine, want: ratio_setting.ModelPurposeImage},
		{name: "Midjourney 视频", path: "/mj/submit/video", relayMode: relayconstant.RelayModeMidjourneyVideo, want: ratio_setting.ModelPurposeVideo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, requestModelPurpose(tt.path, tt.relayMode))
		})
	}
}

func TestDistribute_whenModelPurposeDoesNotMatchEndpoint(t *testing.T) {
	require.NoError(t, appI18n.Init())
	previous := ratio_setting.ModelPurpose2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPurposeByJSONString(previous))
	})
	require.NoError(t, ratio_setting.UpdateModelPurposeByJSONString(`{"gpt-image-2":["image"]}`))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/chat/completions", Distribute(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-image-2"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "invalid_request", response.Error.Code)
}
