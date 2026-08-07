package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	channelRacePending int32 = iota
	channelRaceReady
	channelRaceTimedOut
)

type channelRaceAttempt struct {
	index  int
	member model.ChannelMember
	ctx    *gin.Context
	info   *relaycommon.RelayInfo
	writer *channelRaceResponseWriter
	cancel context.CancelCauseFunc
	timer  *time.Timer
	state  atomic.Int32
}

type channelRaceEvent struct {
	attempt *channelRaceAttempt
	ready   bool
	err     *types.NewAPIError
}

func newChannelRaceAttempt(request channelGroupRaceRequest, index int, member model.ChannelMember, events chan<- channelRaceEvent) (*channelRaceAttempt, *types.NewAPIError) {
	attemptInfo, err := request.info.CloneForAttempt()
	if err != nil {
		return nil, types.NewError(fmt.Errorf("复制渠道组请求: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	requestCtx, cancel := context.WithCancelCause(request.ctx.Request.Context())
	attemptCtx := request.ctx.Copy()
	attemptCtx.Request = request.ctx.Request.Clone(requestCtx)
	service.MarkChannelRaceAttempt(attemptCtx)
	resolved := model.ResolveChannelMember(request.channel, &member)
	if apiErr := middleware.SetupContextForSelectedChannel(attemptCtx, resolved, request.info.OriginModelName); apiErr != nil {
		cancel(apiErr)
		return nil, apiErr
	}
	attempt := &channelRaceAttempt{
		index:  index,
		member: member,
		ctx:    attemptCtx,
		info:   attemptInfo,
		cancel: cancel,
	}
	attempt.writer = newChannelRaceResponseWriter(requestCtx, request.ctx.Writer, func() {
		if !attempt.state.CompareAndSwap(channelRacePending, channelRaceReady) {
			return
		}
		if attempt.timer != nil {
			attempt.timer.Stop()
		}
		events <- channelRaceEvent{attempt: attempt, ready: true}
	})
	attemptCtx.Writer = attempt.writer
	timeout := time.Duration(request.channel.GetResponseTimeout()) * time.Second
	if timeout <= 0 {
		timeout = channelRaceDefaultTimeout
	}
	attempt.timer = time.AfterFunc(timeout, func() {
		if attempt.state.CompareAndSwap(channelRacePending, channelRaceTimedOut) {
			cancel(errChannelRaceTimedOut)
		}
	})
	return attempt, nil
}

func runChannelRaceAttempt(request channelGroupRaceRequest, attempt *channelRaceAttempt, events chan<- channelRaceEvent) {
	finishInflight := service.BeginChannelInflight(request.channel.Id)
	defer finishInflight()
	defer attempt.cancel(nil)
	apiErr := request.handler(attempt.ctx, attempt.info)
	if attempt.timer != nil {
		attempt.timer.Stop()
	}
	if attempt.state.Load() == channelRaceTimedOut {
		timeout := time.Duration(request.channel.GetResponseTimeout()) * time.Second
		if timeout <= 0 {
			timeout = channelRaceDefaultTimeout
		}
		apiErr = types.NewErrorWithStatusCode(
			&relaycommon.ChannelResponseTimeoutError{Timeout: timeout},
			types.ErrorCodeChannelResponseTimeExceeded,
			http.StatusGatewayTimeout,
		)
	}
	if apiErr == nil && !attempt.writer.Written() {
		apiErr = types.NewError(errors.New("渠道组成员未产生可转发响应"), types.ErrorCodeBadResponse)
	}
	events <- channelRaceEvent{attempt: attempt, err: apiErr}
}
