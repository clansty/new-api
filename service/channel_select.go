package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type RetryParam struct {
	Ctx                     *gin.Context
	TokenGroup              string
	ModelName               string
	Retry                   *int
	FailedChannelIDs        map[int]struct{}
	RequireDifferentChannel bool
	resetNextTry            bool
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

func (p *RetryParam) AddFailedChannel(channelID int) {
	if channelID <= 0 {
		return
	}
	if p.FailedChannelIDs == nil {
		p.FailedChannelIDs = make(map[int]struct{})
	}
	p.FailedChannelIDs[channelID] = struct{}{}
}

// CacheGetRandomSatisfiedChannel selects a channel for the current retry.
// It avoids channels that already failed in this relay request when another
// channel is available, preferring remaining channels in the highest priority
// tier before falling through to lower priorities.
//
// For the "auto" token group, ContextKeyAutoGroupIndex tracks the current auto
// group. Cross-group retry moves to the next auto group only after the current
// group has used the configured retry budget.
func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	var channel *model.Channel
	var err error
	selectGroup := param.TokenGroup
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)

	// 解析该模型的有效重试次数与优先级选路策略（未单独配置则回退到全局）
	effectiveRetryTimes := operation_setting.GetModelRetryTimes(param.ModelName, common.RetryTimes)
	nextPriorityOnFailure := operation_setting.UseNextPriorityOnFailure(param.ModelName)

	if param.TokenGroup == "auto" {
		if len(setting.GetAutoGroups()) == 0 {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}
		autoGroups := GetUserAutoGroup(userGroup)

		// startGroupIndex: the group index to start searching from
		// startGroupIndex: 开始搜索的分组索引
		startGroupIndex := 0
		crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)

		if lastGroupIndex, exists := common.GetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
			if idx, ok := lastGroupIndex.(int); ok {
				startGroupIndex = idx
			}
		}

		for i := startGroupIndex; i < len(autoGroups); i++ {
			autoGroup := autoGroups[i]
			groupRetry := param.GetRetry()
			if i > startGroupIndex {
				groupRetry = 0
			}
			logger.LogDebug(param.Ctx, "Auto selecting group: %s, groupRetry: %d", autoGroup, groupRetry)

			if param.RequireDifferentChannel {
				channel, _ = model.GetRandomSatisfiedChannelExcludingFailedStrict(autoGroup, param.ModelName, param.FailedChannelIDs, nextPriorityOnFailure)
			} else {
				channel, _ = model.GetRandomSatisfiedChannelExcludingFailed(autoGroup, param.ModelName, param.FailedChannelIDs, nextPriorityOnFailure)
			}
			if channel == nil {
				logger.LogDebug(param.Ctx, "No available channel in group %s for model %s at groupRetry %d, trying next group", autoGroup, param.ModelName, groupRetry)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, 0)
				param.SetRetry(0)
				continue
			}
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, autoGroup)
			selectGroup = autoGroup
			logger.LogDebug(param.Ctx, "Auto selected group: %s", autoGroup)

			if crossGroupRetry && groupRetry >= effectiveRetryTimes {
				logger.LogDebug(param.Ctx, "Current group %s retries exhausted (groupRetry=%d >= RetryTimes=%d), preparing switch to next group for next retry", autoGroup, groupRetry, effectiveRetryTimes)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				param.SetRetry(0)
				param.ResetRetryNextTry()
			} else {
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
			}
			break
		}
	} else {
		if param.RequireDifferentChannel {
			channel, err = model.GetRandomSatisfiedChannelExcludingFailedStrict(param.TokenGroup, param.ModelName, param.FailedChannelIDs, nextPriorityOnFailure)
		} else {
			channel, err = model.GetRandomSatisfiedChannelExcludingFailed(param.TokenGroup, param.ModelName, param.FailedChannelIDs, nextPriorityOnFailure)
		}
		if err != nil {
			return nil, param.TokenGroup, err
		}
	}
	return channel, selectGroup, nil
}
