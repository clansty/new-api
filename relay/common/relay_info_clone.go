package common

import (
	"fmt"
	"reflect"
	"time"

	appcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
)

func (info *RelayInfo) CloneForAttempt() (*RelayInfo, error) {
	if info == nil {
		return nil, fmt.Errorf("relay info is nil")
	}
	request, err := cloneRelayRequest(info.Request)
	if err != nil {
		return nil, err
	}
	clone := *info
	clone.Request = request
	clone.ChannelMeta = nil
	clone.StreamStatus = nil
	clone.FirstResponseTime = info.StartTime.Add(-time.Second)
	clone.isFirstResponse = true
	clone.SendResponseCount = 0
	clone.ReceivedResponseCount = 0
	clone.ParamOverrideAudit = nil
	clone.RequestConversionChain = append([]types.RelayFormat(nil), info.RequestConversionChain...)
	clone.ThinkingContentInfo = ThinkingContentInfo{IsFirstThinkingContent: true}
	if info.ClaudeConvertInfo != nil {
		claudeConvertInfo := *info.ClaudeConvertInfo
		if info.ClaudeConvertInfo.Usage != nil {
			usage := *info.ClaudeConvertInfo.Usage
			claudeConvertInfo.Usage = &usage
		}
		clone.ClaudeConvertInfo = &claudeConvertInfo
	}
	if info.RuntimeHeadersOverride != nil {
		clone.RuntimeHeadersOverride = make(map[string]interface{}, len(info.RuntimeHeadersOverride))
		for key, value := range info.RuntimeHeadersOverride {
			clone.RuntimeHeadersOverride[key] = value
		}
	}
	if info.ResponsesUsageInfo != nil {
		clone.ResponsesUsageInfo = &ResponsesUsageInfo{BuiltInTools: make(map[string]*BuildInToolInfo, len(info.ResponsesUsageInfo.BuiltInTools))}
		for key, value := range info.ResponsesUsageInfo.BuiltInTools {
			if value == nil {
				clone.ResponsesUsageInfo.BuiltInTools[key] = nil
				continue
			}
			copied := *value
			clone.ResponsesUsageInfo.BuiltInTools[key] = &copied
		}
	}
	return &clone, nil
}

func cloneRelayRequest(request dto.Request) (dto.Request, error) {
	if request == nil {
		return nil, nil
	}
	requestType := reflect.TypeOf(request)
	if requestType.Kind() != reflect.Pointer {
		return nil, fmt.Errorf("relay request must be a pointer, got %T", request)
	}
	payload, err := appcommon.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal relay request: %w", err)
	}
	target := reflect.New(requestType.Elem()).Interface()
	if err := appcommon.Unmarshal(payload, target); err != nil {
		return nil, fmt.Errorf("unmarshal relay request: %w", err)
	}
	cloned, ok := target.(dto.Request)
	if !ok {
		return nil, fmt.Errorf("cloned relay request does not implement dto.Request: %T", target)
	}
	return cloned, nil
}
