package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	if channelError.MemberId > 0 {
		disableChannelMember(channelError, reason)
		return
	}
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		if channel, err := model.CacheGetChannel(channelError.ChannelId); err == nil && channel.Status != common.ChannelStatusEnabled {
			_, _ = CancelChannelAffinityForce(channelError.ChannelId)
		}
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
	}
}

func disableChannelMember(channelError types.ChannelError, reason string) {
	common.SysLog(fmt.Sprintf("渠道组「%s」（#%d）成员「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, channelError.MemberName, channelError.MemberId, reason))
	if !channelError.AutoBan {
		return
	}
	success, err := model.UpdateChannelMemberStatus(channelError.ChannelId, channelError.MemberId, common.ChannelStatusAutoDisabled, reason)
	if err != nil {
		common.SysError("自动禁用渠道组成员失败: " + err.Error())
		return
	}
	if !success {
		return
	}
	if channel, err := model.CacheGetChannel(channelError.ChannelId); err == nil && channel.Status != common.ChannelStatusEnabled {
		_, _ = CancelChannelAffinityForce(channelError.ChannelId)
	}
	subject := fmt.Sprintf("渠道组「%s」成员「%s」已被禁用", channelError.ChannelName, channelError.MemberName)
	content := fmt.Sprintf("渠道组「%s」（#%d）成员「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, channelError.MemberName, channelError.MemberId, reason)
	NotifyRootUser(formatNotifyType(channelError.MemberId, common.ChannelStatusAutoDisabled), subject, content)
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

func ShouldDisableChannel(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	var responseTimeoutErr *relaycommon.ChannelResponseTimeoutError
	if errors.As(err, &responseTimeoutErr) {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	return search
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
