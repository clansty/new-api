package common

import (
	"bytes"
	"io"

	appcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"
)

func ShouldPassThroughRequest(info *RelayInfo) bool {
	if info == nil || info.ChannelMeta == nil {
		return false
	}
	if !isPassThroughRelayMode(info.RelayMode) {
		return false
	}
	if info.ChannelType == constant.ChannelTypeAdvancedPassThrough {
		return shouldPassThroughAdvancedRequest(info)
	}
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled {
		return true
	}
	if info.ChannelType == constant.ChannelTypePassThrough {
		return true
	}
	return info.ChannelSetting.PassThroughBodyEnabled
}

func shouldPassThroughAdvancedRequest(info *RelayInfo) bool {
	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions, relayconstant.RelayModeClaudeMessages:
		return true
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		return info.ChannelOtherSettings.AdvancedResponsesSupported
	default:
		return false
	}
}

func isPassThroughRelayMode(relayMode int) bool {
	switch relayMode {
	case relayconstant.RelayModeChatCompletions,
		relayconstant.RelayModeResponses,
		relayconstant.RelayModeResponsesCompact,
		relayconstant.RelayModeClaudeMessages,
		relayconstant.RelayModeGemini:
		return true
	default:
		return false
	}
}

func BuildPassThroughRequestBody(info *RelayInfo, body []byte) ([]byte, error) {
	if info == nil || info.ChannelMeta == nil || info.ChannelMeta.UpstreamModelName == "" {
		return body, nil
	}
	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions,
		relayconstant.RelayModeResponses,
		relayconstant.RelayModeResponsesCompact,
		relayconstant.RelayModeClaudeMessages:
		return sjson.SetBytes(body, "model", info.ChannelMeta.UpstreamModelName)
	default:
		return body, nil
	}
}

func PassThroughRequestBody(c *gin.Context, info *RelayInfo) (io.Reader, error) {
	storage, err := appcommon.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	body, err := storage.Bytes()
	if err != nil {
		return nil, err
	}
	jsonData, err := BuildPassThroughRequestBody(info, body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(jsonData), nil
}
