package controller

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

func TestShouldRetryAndRecord_whenAffinityHasNoChannelSwitchRetryBudget(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(nil)
	retryParam := &service.RetryParam{AffinityChannelID: 7}
	channel := &model.Channel{Id: 7}
	channel.SetOtherSettings(dto.ChannelOtherSettings{InPlaceRetryTimes: 1})
	err := types.NewError(errors.New("upstream failed"), types.ErrorCodeBadResponse)

	require.True(t, shouldRetryAndRecord(c, err, 0, retryParam, channel))
	require.Equal(t, 1, retryParam.InPlaceRetryCount)
	require.Same(t, channel, retryParam.RetryChannel)
}

func TestRetryLoop_preservesFallbackAttemptAfterAffinityInPlaceRetry(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	affinity := model.Channel{
		Name:     "亲和渠道",
		Status:   common.ChannelStatusEnabled,
		Models:   "retry-test-model",
		Group:    "default",
		Priority: common.GetPointer[int64](10),
	}
	affinity.SetOtherSettings(dto.ChannelOtherSettings{InPlaceRetryTimes: 1})
	fallback := model.Channel{
		Name:     "备用渠道",
		Status:   common.ChannelStatusEnabled,
		Models:   "retry-test-model",
		Group:    "default",
		Priority: common.GetPointer[int64](0),
	}
	require.NoError(t, db.Create(&affinity).Error)
	require.NoError(t, db.Create(&fallback).Error)
	require.NoError(t, db.Create([]model.Ability{
		{Group: "default", Model: "retry-test-model", ChannelId: affinity.Id, Enabled: true, Priority: affinity.Priority},
		{Group: "default", Model: "retry-test-model", ChannelId: fallback.Id, Enabled: true, Priority: fallback.Priority},
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(c, constant.ContextKeyChannelId, affinity.Id)
	common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, affinity.GetOtherSettings())
	common.SetContextKey(c, constant.ContextKeyChannelAffinityId, affinity.Id)

	retryParam := &service.RetryParam{
		Ctx:               c,
		TokenGroup:        "default",
		ModelName:         "retry-test-model",
		Retry:             common.GetPointer(0),
		AffinityChannelID: affinity.Id,
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "retry-test-model",
		TokenGroup:      "default",
		UsingGroup:      "default",
	}
	upstreamErr := types.NewError(errors.New("upstream failed"), types.ErrorCodeBadResponse)
	attemptedChannels := make([]int, 0, 3)

	const channelSwitchRetryTimes = 1
	for ; retryParam.GetRetry() <= channelSwitchRetryTimes; retryParam.IncreaseRetry() {
		channel, channelErr := getChannel(c, info, retryParam)
		require.Nil(t, channelErr)
		attemptedChannels = append(attemptedChannels, channel.Id)
		if channel.Id == fallback.Id {
			break
		}
		info.LastError = upstreamErr
		if !shouldRetryAndRecord(c, upstreamErr, channelSwitchRetryTimes-retryParam.GetRetry(), retryParam, channel) {
			break
		}
	}

	require.Equal(t, []int{affinity.Id, affinity.Id, fallback.Id}, attemptedChannels)
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
