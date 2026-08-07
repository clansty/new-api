package service

import (
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

const (
	channelRaceTraceStoreKey = "channel_race_trace_store"
	channelRaceTraceIndexKey = "channel_race_trace_index"
)

type ChannelRaceMemberLog struct {
	MemberId   int    `json:"member_id"`
	MemberName string `json:"member_name"`
}

type ChannelGroupRaceLog struct {
	ChannelId   int                    `json:"channel_id"`
	ChannelName string                 `json:"channel_name"`
	Members     []ChannelRaceMemberLog `json:"members"`
	Winner      *ChannelRaceMemberLog  `json:"winner,omitempty"`
}

type channelRaceTraceStore struct {
	mu     sync.RWMutex
	traces []ChannelGroupRaceLog
}

func BeginChannelRaceTrace(ctx *gin.Context, trace ChannelGroupRaceLog) {
	if ctx == nil {
		return
	}
	trace.Members = append([]ChannelRaceMemberLog(nil), trace.Members...)
	store, ok := ctx.Get(channelRaceTraceStoreKey)
	traceStore, valid := store.(*channelRaceTraceStore)
	if !ok || !valid {
		traceStore = &channelRaceTraceStore{}
		ctx.Set(channelRaceTraceStoreKey, traceStore)
	}
	traceStore.mu.Lock()
	index := len(traceStore.traces)
	traceStore.traces = append(traceStore.traces, trace)
	traceStore.mu.Unlock()
	ctx.Set(channelRaceTraceIndexKey, index)
}

func MarkChannelRaceAttempt(ctx *gin.Context) {
	common.SetContextKey(ctx, constant.ContextKeyChannelRaceAttempt, true)
	common.SetContextKey(ctx, constant.ContextKeyChannelRaceWinner, false)
}

func MarkChannelRaceWinner(ctx *gin.Context) {
	common.SetContextKey(ctx, constant.ContextKeyChannelRaceWinner, true)
	store, exists := ctx.Get(channelRaceTraceStoreKey)
	traceStore, valid := store.(*channelRaceTraceStore)
	if !exists || !valid {
		return
	}
	index := ctx.GetInt(channelRaceTraceIndexKey)
	winner := ChannelRaceMemberLog{
		MemberId:   common.GetContextKeyInt(ctx, constant.ContextKeyChannelMemberId),
		MemberName: common.GetContextKeyString(ctx, constant.ContextKeyChannelMemberName),
	}
	traceStore.mu.Lock()
	defer traceStore.mu.Unlock()
	if index < 0 || index >= len(traceStore.traces) {
		return
	}
	traceStore.traces[index].Winner = &winner
}

func ShouldSkipChannelRaceAccounting(ctx *gin.Context) bool {
	return common.GetContextKeyBool(ctx, constant.ContextKeyChannelRaceAttempt) &&
		!common.GetContextKeyBool(ctx, constant.ContextKeyChannelRaceWinner)
}

func AppendChannelRaceAdminInfo(ctx *gin.Context, adminInfo map[string]interface{}) {
	if ctx == nil || adminInfo == nil {
		return
	}
	store, exists := ctx.Get(channelRaceTraceStoreKey)
	traceStore, valid := store.(*channelRaceTraceStore)
	if !exists || !valid {
		return
	}
	traceStore.mu.RLock()
	defer traceStore.mu.RUnlock()
	if len(traceStore.traces) == 0 {
		return
	}
	traces := make([]ChannelGroupRaceLog, len(traceStore.traces))
	for index := range traceStore.traces {
		traces[index] = traceStore.traces[index]
		traces[index].Members = append([]ChannelRaceMemberLog(nil), traceStore.traces[index].Members...)
		if traceStore.traces[index].Winner != nil {
			winner := *traceStore.traces[index].Winner
			traces[index].Winner = &winner
		}
	}
	adminInfo["channel_group_races"] = traces
}
