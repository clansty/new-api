package controller

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func shouldRetryAndRecord(c *gin.Context, openaiErr *types.NewAPIError, retryTimes int, retryParam *service.RetryParam, channel *model.Channel) bool {
	if openaiErr != nil && openaiErr.GetErrorCode() == types.ErrorCodeChannelResponseTimeExceeded && retryTimes <= 0 {
		return false
	}
	if !shouldRetry(c, openaiErr, retryTimes) {
		return false
	}
	if openaiErr.GetErrorCode() == types.ErrorCodeChannelResponseTimeExceeded || channel.IsGroup {
		retryParam.RequireDifferentChannel = true
	}
	retryParam.AddFailedChannel(channel.Id)
	return true
}

func shouldRetryTaskRelayAndRecord(c *gin.Context, channelID int, taskErr *dto.TaskError, retryTimes int, retryParam *service.RetryParam) bool {
	if !shouldRetryTaskRelay(c, channelID, taskErr, retryTimes) {
		return false
	}
	retryParam.AddFailedChannel(channelID)
	return true
}
