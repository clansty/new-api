package passthrough

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("relay info is nil")
	}
	if !isSupportedRelayFormat(info.RelayFormat) {
		return "", fmt.Errorf("unsupported pass-through relay format: %s", info.RelayFormat)
	}
	requestPath := info.RequestURLPath
	if info.RelayFormat == types.RelayFormatGemini {
		requestPath = rewriteGeminiModelInPath(requestPath, info.OriginModelName, info.UpstreamModelName)
	}
	requestPath = stripClientCredentialQuery(requestPath)
	return relaycommon.GetFullRequestURL(strings.TrimRight(info.ChannelBaseUrl, "/"), requestPath, info.ChannelType), nil
}

func rewriteGeminiModelInPath(requestPath string, originModel string, upstreamModel string) string {
	if originModel == "" || upstreamModel == "" || originModel == upstreamModel {
		return requestPath
	}
	escapedOrigin := url.PathEscape(originModel)
	escapedUpstream := url.PathEscape(upstreamModel)
	return strings.Replace(requestPath, "/models/"+escapedOrigin+":", "/models/"+escapedUpstream+":", 1)
}

func stripClientCredentialQuery(requestPath string) string {
	parsedURL, err := url.Parse(requestPath)
	if err != nil || parsedURL.RawQuery == "" {
		return requestPath
	}
	query := parsedURL.Query()
	query.Del("key")
	query.Del("api_key")
	query.Del("access_token")
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

func isSupportedRelayFormat(format types.RelayFormat) bool {
	switch format {
	case types.RelayFormatClaude,
		types.RelayFormatGemini,
		types.RelayFormatOpenAI,
		types.RelayFormatOpenAIResponses,
		types.RelayFormatOpenAIResponsesCompaction:
		return true
	default:
		return false
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		req.Set("x-api-key", info.ApiKey)
		anthropicVersion := c.Request.Header.Get("anthropic-version")
		if anthropicVersion == "" {
			anthropicVersion = "2023-06-01"
		}
		req.Set("anthropic-version", anthropicVersion)
		claude.CommonClaudeHeadersOperation(c, req, info)
	case types.RelayFormatGemini:
		req.Set("x-goog-api-key", info.ApiKey)
	default:
		req.Set("Authorization", "Bearer "+info.ApiKey)
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("audio pass-through is not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if info == nil {
		return nil, errors.New("relay info is nil")
	}
	if !isSupportedRelayFormat(info.RelayFormat) {
		return nil, fmt.Errorf("unsupported pass-through relay format: %s", info.RelayFormat)
	}
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if info == nil {
		return nil, types.NewError(errors.New("relay info is nil"), types.ErrorCodeInvalidRequest)
	}
	if !isSupportedRelayFormat(info.RelayFormat) {
		return nil, types.NewError(fmt.Errorf("unsupported pass-through relay format: %s", info.RelayFormat), types.ErrorCodeInvalidRequest)
	}
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		return (&claude.Adaptor{}).DoResponse(c, resp, info)
	case types.RelayFormatGemini:
		return (&gemini.Adaptor{}).DoResponse(c, resp, info)
	default:
		return (&openai.Adaptor{}).DoResponse(c, resp, info)
	}
}

func (a *Adaptor) GetModelList() []string {
	return openai.ModelList
}

func (a *Adaptor) GetChannelName() string {
	return "PassThrough"
}
