package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

func MarkChannelRaceAttempt(ctx *gin.Context) {
	common.SetContextKey(ctx, constant.ContextKeyChannelRaceAttempt, true)
	common.SetContextKey(ctx, constant.ContextKeyChannelRaceWinner, false)
}

func MarkChannelRaceWinner(ctx *gin.Context) {
	common.SetContextKey(ctx, constant.ContextKeyChannelRaceWinner, true)
}

func ShouldSkipChannelRaceAccounting(ctx *gin.Context) bool {
	return common.GetContextKeyBool(ctx, constant.ContextKeyChannelRaceAttempt) &&
		!common.GetContextKeyBool(ctx, constant.ContextKeyChannelRaceWinner)
}
