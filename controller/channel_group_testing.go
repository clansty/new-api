package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestChannelGroupMember(c *gin.Context) {
	channel, ok := loadChannelGroup(c)
	if !ok {
		return
	}
	memberId, err := strconv.Atoi(c.Param("member_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	member, err := model.GetChannelMember(channel.Id, memberId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	resolved := model.ResolveChannelMember(channel, member)
	isStream, _ := strconv.ParseBool(c.Query("stream"))
	startedAt := time.Now()
	result := testChannel(resolved, c.Query("model"), c.Query("endpoint_type"), isStream)
	milliseconds := time.Since(startedAt).Milliseconds()
	model.UpdateChannelMemberResponseTime(channel.Id, member.Id, milliseconds)
	if result.localErr != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": result.localErr.Error(), "time": 0.0})
		return
	}
	if result.newAPIError != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":    false,
			"message":    result.newAPIError.Error(),
			"time":       float64(milliseconds) / 1000.0,
			"error_code": result.newAPIError.GetErrorCode(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "time": float64(milliseconds) / 1000.0})
}

func shouldTestChannelMemberAutomatically(member model.ChannelMember, scope channelTestScope, autoRecover bool) bool {
	if member.Status == common.ChannelStatusManuallyDisabled {
		return false
	}
	if member.Status == common.ChannelStatusAutoDisabled {
		return autoRecover
	}
	return scope != channelTestScopeAutoDisabled
}

func testChannelGroupMembersAutomatically(channel *model.Channel, scope channelTestScope, disableThreshold int64) {
	members, err := model.GetChannelMembers(channel.Id, true)
	if err != nil {
		common.SysError(fmt.Sprintf("获取渠道组成员失败: channel_id=%d, error=%v", channel.Id, err))
		return
	}
	for i := range members {
		member := &members[i]
		if !shouldTestChannelMemberAutomatically(*member, scope, channel.GetAutoRecover()) {
			continue
		}
		resolved := model.ResolveChannelMember(channel, member)
		startedAt := time.Now()
		result := testChannel(resolved, "", "", shouldUseStreamForAutomaticChannelTest(resolved))
		milliseconds := time.Since(startedAt).Milliseconds()
		newAPIError := result.newAPIError
		shouldDisable := newAPIError != nil && service.ShouldDisableChannel(newAPIError)
		if common.AutomaticDisableChannelEnabled && !shouldDisable && milliseconds > disableThreshold {
			err := fmt.Errorf("响应时间 %.2fs 超过阈值 %.2fs", float64(milliseconds)/1000.0, float64(disableThreshold)/1000.0)
			newAPIError = types.NewOpenAIError(err, types.ErrorCodeChannelResponseTimeExceeded, http.StatusRequestTimeout)
			shouldDisable = true
		}
		if member.Status == common.ChannelStatusEnabled && shouldDisable && channel.GetAutoBan() {
			processChannelError(result.context, *types.NewChannelMemberError(
				channel.Id,
				channel.Type,
				channel.Name,
				member.Id,
				member.Name,
				channel.GetAutoBan(),
			), newAPIError)
		}
		if member.Status == common.ChannelStatusAutoDisabled && service.ShouldEnableChannel(newAPIError, member.Status) {
			_, _ = model.UpdateChannelMemberStatus(channel.Id, member.Id, common.ChannelStatusEnabled, "")
		}
		model.UpdateChannelMemberResponseTime(channel.Id, member.Id, milliseconds)
		time.Sleep(common.RequestInterval)
	}
}
