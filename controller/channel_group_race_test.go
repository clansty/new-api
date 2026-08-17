package controller

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRelayChannelGroup_returnsFastestMemberAndCancelsLoser(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.ChannelMember{}, &model.Ability{}))
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	channel := model.Channel{
		Name:             "竞速组",
		Type:             constant.ChannelTypeOpenAI,
		Status:           common.ChannelStatusEnabled,
		IsGroup:          true,
		ParallelRequests: 2,
		ResponseTimeout:  common.GetPointer(3),
	}
	require.NoError(t, db.Create(&channel).Error)
	members := []model.ChannelMember{
		{ChannelId: channel.Id, Name: "慢节点", Key: "sk-slow", Status: common.ChannelStatusEnabled},
		{ChannelId: channel.Id, Name: "快节点", Key: "sk-fast", Status: common.ChannelStatusEnabled},
	}
	require.NoError(t, db.Create(&members).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	request := &dto.GeneralOpenAIRequest{Model: "gpt-test"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-test",
		Request:         request,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		StartTime:       time.Now(),
	}
	slowCancelled := make(chan struct{}, 1)
	fastMayWrite := make(chan struct{})

	apiErr := relayChannelGroup(channelGroupRaceRequest{
		ctx:     ctx,
		info:    info,
		channel: &channel,
		handler: func(attemptCtx *gin.Context, _ *relaycommon.RelayInfo) *types.NewAPIError {
			memberName := common.GetContextKeyString(attemptCtx, constant.ContextKeyChannelMemberName)
			if memberName == "慢节点" {
				close(fastMayWrite)
				<-attemptCtx.Request.Context().Done()
				slowCancelled <- struct{}{}
				return types.NewError(context.Cause(attemptCtx.Request.Context()), types.ErrorCodeDoRequestFailed)
			}
			<-fastMayWrite
			if _, err := attemptCtx.Writer.Write([]byte("data: fastest\n\n")); err != nil {
				return types.NewError(err, types.ErrorCodeBadResponse)
			}
			return nil
		},
	})

	require.Nil(t, apiErr)
	require.Equal(t, "data: fastest\n\n", recorder.Body.String())
	select {
	case <-slowCancelled:
	default:
		t.Fatal("慢节点没有收到取消信号")
	}
	require.Equal(t, members[1].Id, common.GetContextKeyInt(ctx, constant.ContextKeyChannelMemberId))
	adminInfo := make(map[string]interface{})
	service.AppendChannelRaceAdminInfo(ctx, adminInfo)
	traces, ok := adminInfo["channel_group_races"].([]service.ChannelGroupRaceLog)
	require.True(t, ok)
	require.Len(t, traces, 1)
	require.Equal(t, channel.Id, traces[0].ChannelId)
	require.Equal(t, channel.Name, traces[0].ChannelName)
	require.ElementsMatch(t, []service.ChannelRaceMemberLog{
		{MemberId: members[0].Id, MemberName: members[0].Name},
		{MemberId: members[1].Id, MemberName: members[1].Name},
	}, traces[0].Members)
	require.Equal(t, &service.ChannelRaceMemberLog{MemberId: members[1].Id, MemberName: members[1].Name}, traces[0].Winner)
}

func TestRelayChannelGroup_affinitySuccessDoesNotStartOtherMembers(t *testing.T) {
	channel, members := setupChannelGroupRaceTest(t, 2, time.Second)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyChannelAffinityMemberId, members[0].Id)
	info := channelGroupRaceInfo()
	otherStarted := make(chan struct{}, 1)

	apiErr := relayChannelGroup(channelGroupRaceRequest{
		ctx:     ctx,
		info:    info,
		channel: channel,
		handler: func(attemptCtx *gin.Context, _ *relaycommon.RelayInfo) *types.NewAPIError {
			if common.GetContextKeyInt(attemptCtx, constant.ContextKeyChannelMemberId) != members[0].Id {
				otherStarted <- struct{}{}
				return nil
			}
			_, err := attemptCtx.Writer.Write([]byte("data: affinity\n\n"))
			if err != nil {
				return types.NewError(err, types.ErrorCodeBadResponse)
			}
			return nil
		},
	})

	require.Nil(t, apiErr)
	require.Equal(t, "data: affinity\n\n", recorder.Body.String())
	select {
	case <-otherStarted:
		t.Fatal("亲和成员成功前不应启动其他成员")
	default:
	}
}

func TestRelayChannelGroup_affinityDelayStartsOtherMembers(t *testing.T) {
	channel, members := setupChannelGroupRaceTest(t, 2, 10*time.Millisecond)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyChannelAffinityMemberId, members[0].Id)
	info := channelGroupRaceInfo()
	otherStarted := make(chan struct{}, 1)

	apiErr := relayChannelGroup(channelGroupRaceRequest{
		ctx:     ctx,
		info:    info,
		channel: channel,
		handler: func(attemptCtx *gin.Context, _ *relaycommon.RelayInfo) *types.NewAPIError {
			if common.GetContextKeyInt(attemptCtx, constant.ContextKeyChannelMemberId) == members[0].Id {
				<-attemptCtx.Request.Context().Done()
				return types.NewError(context.Cause(attemptCtx.Request.Context()), types.ErrorCodeDoRequestFailed)
			}
			otherStarted <- struct{}{}
			_, err := attemptCtx.Writer.Write([]byte("data: fallback\n\n"))
			if err != nil {
				return types.NewError(err, types.ErrorCodeBadResponse)
			}
			return nil
		},
	})

	require.Nil(t, apiErr)
	require.Equal(t, "data: fallback\n\n", recorder.Body.String())
	select {
	case <-otherStarted:
	default:
		t.Fatal("等待窗口到期后未启动其他成员")
	}
}

func TestRelayChannelGroup_affinityFailureStartsOtherMembersImmediately(t *testing.T) {
	channel, members := setupChannelGroupRaceTest(t, 2, 10*time.Second)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyChannelAffinityMemberId, members[0].Id)
	info := channelGroupRaceInfo()
	otherStarted := make(chan struct{}, 1)

	done := make(chan *types.NewAPIError, 1)
	go func() {
		done <- relayChannelGroup(channelGroupRaceRequest{
			ctx:     ctx,
			info:    info,
			channel: channel,
			handler: func(attemptCtx *gin.Context, _ *relaycommon.RelayInfo) *types.NewAPIError {
				if common.GetContextKeyInt(attemptCtx, constant.ContextKeyChannelMemberId) == members[0].Id {
					return types.NewError(errors.New("亲和成员失败"), types.ErrorCodeBadResponse)
				}
				otherStarted <- struct{}{}
				_, err := attemptCtx.Writer.Write([]byte("data: immediate fallback\n\n"))
				if err != nil {
					return types.NewError(err, types.ErrorCodeBadResponse)
				}
				return nil
			},
		})
	}()

	select {
	case <-otherStarted:
	case <-time.After(time.Second):
		t.Fatal("亲和成员失败后未立即启动其他成员")
	}
	require.Nil(t, <-done)
	require.Equal(t, "data: immediate fallback\n\n", recorder.Body.String())
}

func setupChannelGroupRaceTest(t *testing.T, parallel int, affinityDelay time.Duration) (*model.Channel, []model.ChannelMember) {
	t.Helper()
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.ChannelMember{}, &model.Ability{}))
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })
	channel := &model.Channel{
		Name:                  "亲和竞速组",
		Type:                  constant.ChannelTypeOpenAI,
		Status:                common.ChannelStatusEnabled,
		IsGroup:               true,
		ParallelRequests:      parallel,
		AffinityParallelDelay: int(affinityDelay / time.Millisecond),
		ResponseTimeout:       common.GetPointer(3),
	}
	require.NoError(t, db.Create(channel).Error)
	members := []model.ChannelMember{
		{ChannelId: channel.Id, Name: "亲和成员", Key: "sk-affinity", Status: common.ChannelStatusEnabled},
		{ChannelId: channel.Id, Name: "补充成员", Key: "sk-fallback", Status: common.ChannelStatusEnabled},
	}
	require.NoError(t, db.Create(&members).Error)
	return channel, members
}

func channelGroupRaceInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: "gpt-test",
		Request:         &dto.GeneralOpenAIRequest{Model: "gpt-test"},
		RelayMode:       relayconstant.RelayModeChatCompletions,
		StartTime:       time.Now(),
	}
}

func TestCleanupChannelRaceAttempts_cancelsCreatedAttempts(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest("POST", "/", nil)
	writer := newChannelRaceResponseWriter(ctx, ginCtx.Writer, func() {})
	attemptCtx, attemptCancel := context.WithCancelCause(ctx)
	attempt := &channelRaceAttempt{
		ctx:    ginCtx,
		writer: writer,
		cancel: attemptCancel,
	}

	cleanupChannelRaceAttempts([]*channelRaceAttempt{attempt}, errChannelRaceAborted)

	select {
	case <-attemptCtx.Done():
	default:
		t.Fatal("已创建的竞速 attempt 未收到取消信号")
	}
	_, err := writer.Write([]byte("data"))
	require.ErrorIs(t, err, errChannelRaceLost)
}

func TestRelayChannelGroup_isolatesClaudeConversionStatePerAttempt(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.ChannelMember{}, &model.Ability{}))
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	channel := model.Channel{
		Name:             "Claude 状态隔离组",
		Type:             constant.ChannelTypeOpenAI,
		Status:           common.ChannelStatusEnabled,
		IsGroup:          true,
		ParallelRequests: 2,
	}
	require.NoError(t, db.Create(&channel).Error)
	members := []model.ChannelMember{
		{ChannelId: channel.Id, Name: "状态成员 A", Key: "sk-a", Status: common.ChannelStatusEnabled},
		{ChannelId: channel.Id, Name: "状态成员 B", Key: "sk-b", Status: common.ChannelStatusEnabled},
	}
	require.NoError(t, db.Create(&members).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	info := &relaycommon.RelayInfo{
		OriginModelName:   "claude-test",
		Request:           &dto.ClaudeRequest{},
		RelayMode:         relayconstant.RelayModeClaudeMessages,
		RelayFormat:       types.RelayFormatClaude,
		StartTime:         time.Now(),
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{},
	}
	memberASet := make(chan struct{})
	releaseMemberA := make(chan struct{})
	memberBObservedDone := make(chan bool, 1)

	apiErr := relayChannelGroup(channelGroupRaceRequest{
		ctx:     ctx,
		info:    info,
		channel: &channel,
		handler: func(attemptCtx *gin.Context, attemptInfo *relaycommon.RelayInfo) *types.NewAPIError {
			memberName := common.GetContextKeyString(attemptCtx, constant.ContextKeyChannelMemberName)
			if memberName == members[0].Name {
				attemptInfo.ClaudeConvertInfo.Done = true
				close(memberASet)
				<-releaseMemberA
			} else {
				<-memberASet
				memberBObservedDone <- attemptInfo.ClaudeConvertInfo.Done
				close(releaseMemberA)
			}
			if _, writeErr := attemptCtx.Writer.Write([]byte("data: member\n\n")); writeErr != nil {
				return types.NewError(writeErr, types.ErrorCodeBadResponse)
			}
			return nil
		},
	})

	require.Nil(t, apiErr)
	require.False(t, <-memberBObservedDone)
}

func TestRelayChannelGroup_twoFailuresOnlyWaitsGraceForPendingMember(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.ChannelMember{}, &model.Ability{}))
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	channel := model.Channel{
		Name:             "失败仲裁组",
		Type:             constant.ChannelTypeOpenAI,
		Status:           common.ChannelStatusEnabled,
		IsGroup:          true,
		ParallelRequests: 3,
		ResponseTimeout:  common.GetPointer(3),
	}
	require.NoError(t, db.Create(&channel).Error)
	members := []model.ChannelMember{
		{ChannelId: channel.Id, Name: "失败一", Key: "sk-fail-1", Status: common.ChannelStatusEnabled},
		{ChannelId: channel.Id, Name: "失败二", Key: "sk-fail-2", Status: common.ChannelStatusEnabled},
		{ChannelId: channel.Id, Name: "代理超时", Key: "sk-timeout", Status: common.ChannelStatusEnabled},
	}
	require.NoError(t, db.Create(&members).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-test",
		Request:         &dto.GeneralOpenAIRequest{Model: "gpt-test"},
		RelayMode:       relayconstant.RelayModeChatCompletions,
		StartTime:       time.Now(),
	}
	proxyCancelled := make(chan error, 1)
	startedAt := time.Now()

	apiErr := relayChannelGroup(channelGroupRaceRequest{
		ctx:          ctx,
		info:         info,
		channel:      &channel,
		failureGrace: 50 * time.Millisecond,
		handler: func(attemptCtx *gin.Context, _ *relaycommon.RelayInfo) *types.NewAPIError {
			if common.GetContextKeyString(attemptCtx, constant.ContextKeyChannelMemberName) == "代理超时" {
				<-attemptCtx.Request.Context().Done()
				proxyCancelled <- context.Cause(attemptCtx.Request.Context())
				return types.NewError(context.Cause(attemptCtx.Request.Context()), types.ErrorCodeDoRequestFailed)
			}
			return types.NewError(errors.New("upstream rejected"), types.ErrorCodeBadResponse)
		},
	})

	require.NotNil(t, apiErr)
	require.GreaterOrEqual(t, time.Since(startedAt), 50*time.Millisecond)
	require.Less(t, time.Since(startedAt), 500*time.Millisecond)
	select {
	case cause := <-proxyCancelled:
		require.ErrorIs(t, cause, errChannelRaceGraceExpired)
	default:
		t.Fatal("代理超时候选没有在失败仲裁窗口后被取消")
	}
}

func TestNewChannelRaceAttempt_usesResponseTimeoutOnlyForStreamingRequests(t *testing.T) {
	channel := model.Channel{
		Id:              1,
		Name:            "非流式竞速组",
		Type:            constant.ChannelTypeOpenAI,
		Status:          common.ChannelStatusEnabled,
		IsGroup:         true,
		ResponseTimeout: common.GetPointer(1),
	}
	member := model.ChannelMember{
		Id:        1,
		ChannelId: channel.Id,
		Name:      "成员一",
		Key:       "sk-member",
		Status:    common.ChannelStatusEnabled,
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-test",
		Request:         &dto.GeneralOpenAIRequest{Model: "gpt-test"},
		RelayMode:       relayconstant.RelayModeChatCompletions,
		IsStream:        false,
	}
	events := make(chan channelRaceEvent, 2)

	attempt, apiErr := newChannelRaceAttempt(channelGroupRaceRequest{
		ctx:     ctx,
		info:    info,
		channel: &channel,
	}, 0, member, events)
	require.Nil(t, apiErr)
	t.Cleanup(func() {
		if attempt.timer != nil {
			attempt.timer.Stop()
		}
		attempt.cancel(nil)
	})

	require.Nil(t, attempt.timer)

	info.IsStream = true
	streamAttempt, apiErr := newChannelRaceAttempt(channelGroupRaceRequest{
		ctx:     ctx,
		info:    info,
		channel: &channel,
	}, 1, member, events)
	require.Nil(t, apiErr)
	t.Cleanup(func() {
		if streamAttempt.timer != nil {
			streamAttempt.timer.Stop()
		}
		streamAttempt.cancel(nil)
	})

	require.NotNil(t, streamAttempt.timer)
}

func TestGetChannel_retryAfterGroupFailureSelectsDifferentChannel(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	highPriority := int64(10)
	lowPriority := int64(0)
	group := model.Channel{
		Name:     "失败渠道组",
		Type:     constant.ChannelTypeOpenAI,
		Status:   common.ChannelStatusEnabled,
		Models:   "qa-race-model",
		Group:    "default",
		Priority: &highPriority,
		IsGroup:  true,
	}
	fallback := model.Channel{
		Name:     "备用渠道",
		Type:     constant.ChannelTypeOpenAI,
		Key:      "sk-fallback",
		Status:   common.ChannelStatusEnabled,
		Models:   "qa-race-model",
		Group:    "default",
		Priority: &lowPriority,
	}
	require.NoError(t, db.Create(&group).Error)
	require.NoError(t, db.Create(&fallback).Error)
	require.NoError(t, db.Create([]model.Ability{
		{Group: "default", Model: "qa-race-model", ChannelId: group.Id, Enabled: true, Priority: &highPriority},
		{Group: "default", Model: "qa-race-model", ChannelId: fallback.Id, Enabled: true, Priority: &lowPriority},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, group.Id)
	common.SetContextKey(ctx, constant.ContextKeyChannelIsGroup, true)

	retry := 1
	retryParam := &service.RetryParam{
		Ctx:                     ctx,
		TokenGroup:              "default",
		ModelName:               "qa-race-model",
		Retry:                   &retry,
		FailedChannelIDs:        map[int]struct{}{group.Id: {}},
		RequireDifferentChannel: true,
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "qa-race-model",
		TokenGroup:      "default",
		UsingGroup:      "default",
	}

	channel, apiErr := getChannel(ctx, info, retryParam)

	require.Nil(t, apiErr)
	require.Equal(t, fallback.Id, channel.Id)

	resetRecorder := httptest.NewRecorder()
	resetCtx, _ := gin.CreateTestContext(resetRecorder)
	resetCtx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(resetCtx, constant.ContextKeyChannelId, group.Id)
	common.SetContextKey(resetCtx, constant.ContextKeyChannelIsGroup, true)
	resetRetry := 0
	resetRetryParam := &service.RetryParam{
		Ctx:                     resetCtx,
		TokenGroup:              "default",
		ModelName:               "qa-race-model",
		Retry:                   &resetRetry,
		FailedChannelIDs:        map[int]struct{}{group.Id: {}},
		RequireDifferentChannel: true,
	}
	resetInfo := &relaycommon.RelayInfo{
		OriginModelName: "qa-race-model",
		TokenGroup:      "default",
		UsingGroup:      "default",
	}

	channel, apiErr = getChannel(resetCtx, resetInfo, resetRetryParam)

	require.Nil(t, apiErr)
	require.Equal(t, fallback.Id, channel.Id)
}
