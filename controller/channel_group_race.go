package controller

import (
	"context"
	"errors"
	"fmt"
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
	"golang.org/x/sync/errgroup"
)

const (
	channelRaceFailureGrace   = 2 * time.Second
	channelRaceDefaultTimeout = 30 * time.Second
)

var (
	errChannelRaceGraceExpired = errors.New("渠道组失败仲裁窗口已结束")
	errChannelRaceTimedOut     = errors.New("渠道组成员响应超时")
	errChannelRaceAborted      = errors.New("渠道组请求已中止")
)

type channelGroupRaceRequest struct {
	ctx          *gin.Context
	info         *relaycommon.RelayInfo
	channel      *model.Channel
	failureGrace time.Duration
	handler      func(*gin.Context, *relaycommon.RelayInfo) *types.NewAPIError
}

func relayChannelGroup(request channelGroupRaceRequest) *types.NewAPIError {
	parallel := request.channel.ParallelRequests
	if parallel < 1 {
		parallel = 1
	}
	if parallel > 4 {
		parallel = 4
	}
	members, err := model.SelectEnabledChannelMembers(request.channel.Id, parallel)
	if err != nil {
		return types.NewError(fmt.Errorf("选择渠道组成员: %w", err), types.ErrorCodeGetChannelFailed)
	}
	if len(members) == 0 {
		return types.NewError(errors.New("渠道组没有可用成员"), types.ErrorCodeGetChannelFailed)
	}
	raceMembers := make([]service.ChannelRaceMemberLog, 0, len(members))
	for _, member := range members {
		raceMembers = append(raceMembers, service.ChannelRaceMemberLog{
			MemberId:   member.Id,
			MemberName: member.Name,
		})
	}
	service.BeginChannelRaceTrace(request.ctx, service.ChannelGroupRaceLog{
		ChannelId:   request.channel.Id,
		ChannelName: request.channel.Name,
		Members:     raceMembers,
	})

	events := make(chan channelRaceEvent, len(members)*2)
	attempts := make([]*channelRaceAttempt, 0, len(members))
	group := &errgroup.Group{}
	for index := range members {
		attempt, apiErr := newChannelRaceAttempt(request, index, members[index], events)
		if apiErr != nil {
			cleanupChannelRaceAttempts(attempts, errChannelRaceAborted)
			_ = group.Wait()
			return apiErr
		}
		attempts = append(attempts, attempt)
		group.Go(func() error {
			runChannelRaceAttempt(request, attempt, events)
			return nil
		})
	}

	result := coordinateChannelRace(request, attempts, events)
	_ = group.Wait()
	return result
}

func coordinateChannelRace(request channelGroupRaceRequest, attempts []*channelRaceAttempt, events <-chan channelRaceEvent) *types.NewAPIError {
	remaining := len(attempts)
	winnerIndex := -1
	failureCount := 0
	var lastFailure *types.NewAPIError
	var winnerResult *types.NewAPIError
	var graceTimer *time.Timer
	var grace <-chan time.Time

	for remaining > 0 {
		select {
		case event := <-events:
			if event.ready {
				if winnerIndex < 0 {
					winnerIndex = event.attempt.index
					service.MarkChannelRaceWinner(event.attempt.ctx)
					event.attempt.writer.Decide(true)
					cancelLosingChannelRaceAttempts(attempts, winnerIndex, errChannelRaceLost)
					if graceTimer != nil {
						graceTimer.Stop()
						grace = nil
					}
				} else if event.attempt.index != winnerIndex {
					event.attempt.writer.Decide(false)
				}
				continue
			}

			remaining--
			if event.attempt.index == winnerIndex {
				winnerResult = event.err
				continue
			}
			cause := context.Cause(event.attempt.ctx.Request.Context())
			if errors.Is(cause, errChannelRaceLost) || errors.Is(cause, errChannelRaceGraceExpired) || errors.Is(cause, errChannelRaceAborted) {
				continue
			}
			if event.err == nil {
				continue
			}
			lastFailure = event.err
			failureCount++
			processChannelRaceFailure(request.channel, event.attempt, event.err)
			if types.IsSkipRetryError(event.err) && winnerIndex < 0 {
				cancelLosingChannelRaceAttempts(attempts, -1, errChannelRaceAborted)
			}
			if failureCount >= 2 && remaining > 0 && winnerIndex < 0 && graceTimer == nil {
				failureGrace := request.failureGrace
				if failureGrace <= 0 {
					failureGrace = channelRaceFailureGrace
				}
				graceTimer = time.NewTimer(failureGrace)
				grace = graceTimer.C
			}
		case <-grace:
			grace = nil
			cancelLosingChannelRaceAttempts(attempts, -1, errChannelRaceGraceExpired)
		}
	}
	if graceTimer != nil {
		graceTimer.Stop()
	}
	if winnerIndex >= 0 {
		winner := attempts[winnerIndex]
		common.SetContextKey(request.ctx, constant.ContextKeyChannelMemberId, winner.member.Id)
		common.SetContextKey(request.ctx, constant.ContextKeyChannelMemberName, winner.member.Name)
		if winnerResult != nil {
			processChannelRaceFailure(request.channel, winner, winnerResult)
		}
		return winnerResult
	}
	if lastFailure != nil {
		return lastFailure
	}
	return types.NewError(errors.New("渠道组请求全部失败"), types.ErrorCodeBadResponse)
}

func cancelLosingChannelRaceAttempts(attempts []*channelRaceAttempt, winnerIndex int, cause error) {
	for _, attempt := range attempts {
		if attempt.index == winnerIndex {
			continue
		}
		attempt.writer.Decide(false)
		attempt.cancel(cause)
	}
}

func cleanupChannelRaceAttempts(attempts []*channelRaceAttempt, cause error) {
	for _, attempt := range attempts {
		attempt.writer.Decide(false)
		attempt.cancel(cause)
	}
}

func processChannelRaceFailure(channel *model.Channel, attempt *channelRaceAttempt, apiErr *types.NewAPIError) {
	channelError := types.NewChannelMemberError(
		channel.Id,
		channel.Type,
		channel.Name,
		attempt.member.Id,
		attempt.member.Name,
		channel.GetAutoBan(),
	)
	processChannelError(attempt.ctx, *channelError, apiErr)
}

func supportsChannelGroupRace(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions,
		relayconstant.RelayModeCompletions,
		relayconstant.RelayModeResponses,
		relayconstant.RelayModeResponsesCompact,
		relayconstant.RelayModeClaudeMessages:
		return true
	case relayconstant.RelayModeGemini:
		_, isChat := info.Request.(*dto.GeminiChatRequest)
		return isChat
	default:
		return false
	}
}
