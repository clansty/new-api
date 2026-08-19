package controller

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func shouldRetryAndRecord(c *gin.Context, openaiErr *types.NewAPIError, retryTimes int, retryParam *service.RetryParam, channel *model.Channel) bool {
	if retryParam.AffinityChannelID == channel.Id && retryParam.InPlaceRetryCount < channel.GetOtherSettings().InPlaceRetryTimes && shouldRetry(c, openaiErr, 1) && (!service.ShouldDisableChannelForRequest(c, openaiErr) || !channel.GetAutoBan()) {
		retryParam.InPlaceRetryCount++
		retryParam.SetRetryChannel(channel)
		return true
	}
	if openaiErr != nil && openaiErr.GetErrorCode() == types.ErrorCodeChannelResponseTimeExceeded && retryTimes <= 0 {
		return false
	}
	if !shouldRetry(c, openaiErr, retryTimes) {
		return false
	}
	retryParam.ClearRetryChannel()
	if openaiErr.GetErrorCode() == types.ErrorCodeChannelResponseTimeExceeded || channel.IsGroup {
		retryParam.RequireDifferentChannel = true
	}
	retryParam.AddFailedChannel(channel.Id)
	return true
}

func shouldRetryTaskRelayAndRecord(c *gin.Context, channelID int, taskErr *dto.TaskError, retryTimes int, retryParam *service.RetryParam) bool {
	if retryParam.AffinityChannelID == channelID {
		if channel, err := model.CacheGetChannel(channelID); err == nil && retryParam.InPlaceRetryCount < channel.GetOtherSettings().InPlaceRetryTimes && shouldRetryTaskRelay(c, channelID, taskErr, 1) && (!service.ShouldDisableChannelForRequest(c, types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode)) || !channel.GetAutoBan()) {
			retryParam.InPlaceRetryCount++
			retryParam.SetRetryChannel(channel)
			return true
		}
	}
	if !shouldRetryTaskRelay(c, channelID, taskErr, retryTimes) {
		return false
	}
	retryParam.ClearRetryChannel()
	retryParam.AddFailedChannel(channelID)
	return true
}
