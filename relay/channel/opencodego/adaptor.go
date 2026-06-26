package opencodego

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	openai.Adaptor
}

var ModelList = []string{
	"glm-5.2",
	"glm-5.1",
	"kimi-k2.7",
	"kimi-k2.6",
	"deepseek-v4-pro",
	"deepseek-v4-flash",
	"mimo-v2.5",
	"mimo-v2.5-pro",
	"minimax-m3",
	"minimax-m2.7",
	"minimax-m2.5",
	"qwen3.7-max",
	"qwen3.7-plus",
	"qwen3.6-plus",
}

const ChannelName = "opencode-go"

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if usesMessagesEndpoint(info.UpstreamModelName) {
		return fmt.Sprintf("%s/v1/messages", info.ChannelBaseUrl), nil
	}
	return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	if usesMessagesEndpoint(info.UpstreamModelName) {
		claudeAdaptor := &claude.Adaptor{}
		return claudeAdaptor.SetupRequestHeader(c, req, info)
	}
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if usesMessagesEndpoint(info.UpstreamModelName) {
		claudeAdaptor := &claude.Adaptor{}
		return claudeAdaptor.ConvertOpenAIRequest(c, info, request)
	}
	originalChannelType := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	defer func() {
		info.ChannelType = originalChannelType
	}()
	return a.Adaptor.ConvertOpenAIRequest(c, info, request)
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	openaiRequest, err := service.GeminiToOpenAIRequest(request, info)
	if err != nil {
		return nil, err
	}
	return a.ConvertOpenAIRequest(c, info, openaiRequest)
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if usesMessagesEndpoint(info.UpstreamModelName) {
		return request, nil
	}
	return a.Adaptor.ConvertClaudeRequest(c, info, request)
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	if usesMessagesEndpoint(info.UpstreamModelName) {
		claudeAdaptor := &claude.Adaptor{}
		return claudeAdaptor.ConvertOpenAIResponsesRequest(c, info, request)
	}
	return a.Adaptor.ConvertOpenAIResponsesRequest(c, info, request)
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if usesMessagesEndpoint(info.UpstreamModelName) {
		claudeAdaptor := &claude.Adaptor{}
		return claudeAdaptor.DoResponse(c, resp, info)
	}
	return a.Adaptor.DoResponse(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func usesMessagesEndpoint(modelName string) bool {
	normalizedModelName := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(normalizedModelName, "minimax-") ||
		strings.HasPrefix(normalizedModelName, "qwen")
}
