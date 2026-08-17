package controller

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldRetryAndRecord_whenChannelTimeoutHasRetryRemaining(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(nil)
	retryParam := &service.RetryParam{}
	timeoutErr := types.NewError(errors.New("timeout"), types.ErrorCodeChannelResponseTimeExceeded)

	shouldRetry := shouldRetryAndRecord(c, timeoutErr, 1, retryParam, &model.Channel{Id: 7})

	require.True(t, shouldRetry)
	require.True(t, retryParam.RequireDifferentChannel)
	require.Contains(t, retryParam.FailedChannelIDs, 7)
}

func TestShouldRetryAndRecord_whenChannelTimeoutIsLastAttempt(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(nil)
	retryParam := &service.RetryParam{}
	timeoutErr := types.NewError(errors.New("timeout"), types.ErrorCodeChannelResponseTimeExceeded)

	shouldRetry := shouldRetryAndRecord(c, timeoutErr, 0, retryParam, &model.Channel{Id: 7})

	require.False(t, shouldRetry)
	require.False(t, retryParam.RequireDifferentChannel)
	require.Empty(t, retryParam.FailedChannelIDs)
}

func TestShouldRetryAndRecord_whenChannelGroupFails(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(nil)
	retryParam := &service.RetryParam{}
	upstreamErr := types.NewError(errors.New("upstream failed"), types.ErrorCodeBadResponse)
	channel := &model.Channel{Id: 102, IsGroup: true}

	shouldRetry := shouldRetryAndRecord(c, upstreamErr, 1, retryParam, channel)

	require.True(t, shouldRetry)
	require.True(t, retryParam.RequireDifferentChannel)
	require.Contains(t, retryParam.FailedChannelIDs, 102)
}

func TestShouldRetryAndRecord_whenAffinityChannelHasInPlaceRetries(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(nil)
	retryParam := &service.RetryParam{AffinityChannelID: 7}
	autoBan := 1
	channel := &model.Channel{Id: 7, AutoBan: &autoBan}
	channel.SetOtherSettings(dto.ChannelOtherSettings{InPlaceRetryTimes: 2})
	err := types.NewError(errors.New("upstream failed"), types.ErrorCodeBadResponse)

	require.True(t, shouldRetryAndRecord(c, err, 2, retryParam, channel))
	require.Equal(t, 1, retryParam.InPlaceRetryCount)
	require.Same(t, channel, retryParam.RetryChannel)
	require.Empty(t, retryParam.FailedChannelIDs)

	require.True(t, shouldRetryAndRecord(c, err, 1, retryParam, channel))
	require.Equal(t, 2, retryParam.InPlaceRetryCount)
	require.Same(t, channel, retryParam.RetryChannel)
	require.Empty(t, retryParam.FailedChannelIDs)
}

func TestShouldRetryAndRecord_whenAffinityInPlaceRetriesExhausted(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(nil)
	retryParam := &service.RetryParam{AffinityChannelID: 7, InPlaceRetryCount: 2}
	channel := &model.Channel{Id: 7}
	channel.SetOtherSettings(dto.ChannelOtherSettings{InPlaceRetryTimes: 2})
	err := types.NewError(errors.New("upstream failed"), types.ErrorCodeBadResponse)

	require.True(t, shouldRetryAndRecord(c, err, 1, retryParam, channel))
	require.Nil(t, retryParam.RetryChannel)
	require.Contains(t, retryParam.FailedChannelIDs, 7)
}

func TestShouldRetryAndRecord_whenAffinityErrorAutoDisablesChannel(t *testing.T) {
	original := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = original })
	c, _ := gin.CreateTestContext(nil)
	retryParam := &service.RetryParam{AffinityChannelID: 7}
	autoBan := 1
	channel := &model.Channel{Id: 7, AutoBan: &autoBan}
	channel.SetOtherSettings(dto.ChannelOtherSettings{InPlaceRetryTimes: 2})
	err := types.NewError(errors.New("channel failure"), types.ErrorCode("channel:test"))

	require.True(t, shouldRetryAndRecord(c, err, 1, retryParam, channel))
	require.Nil(t, retryParam.RetryChannel)
	require.Contains(t, retryParam.FailedChannelIDs, 7)
}
