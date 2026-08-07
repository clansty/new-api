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
