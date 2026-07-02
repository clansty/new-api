package model

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type retrySelectionCandidate struct {
	channelID int
	priority  int64
	weight    int
}

// nextPriorityOnFailure 为 true 时，本优先级只要有渠道已失败，就整层跳过、直接尝试下一优先级；
// 为 false（默认）时，在同优先级内继续选择未失败的渠道，本层全部失败后才降级。
func GetRandomSatisfiedChannelExcludingFailed(group string, modelName string, failedChannelIDs map[int]struct{}, nextPriorityOnFailure bool) (*Channel, error) {
	if len(failedChannelIDs) == 0 {
		return GetRandomSatisfiedChannel(group, modelName, 0)
	}
	if !common.MemoryCacheEnabled {
		return getChannelExcludingFailedDB(group, modelName, failedChannelIDs, nextPriorityOnFailure)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	channelIDs := channelIDsForGroupModel(group, modelName)
	if len(channelIDs) == 0 {
		return nil, nil
	}

	candidates := make([]retrySelectionCandidate, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		channel, ok := channelsIDM[channelID]
		if !ok {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelID)
		}
		candidates = append(candidates, retrySelectionCandidate{
			channelID: channel.Id,
			priority:  channel.GetPriority(),
			weight:    channel.GetWeight(),
		})
	}

	channelID, ok, err := selectRetryCandidateID(candidates, failedChannelIDs, nextPriorityOnFailure)
	if err != nil || !ok {
		return nil, err
	}
	channel, ok := channelsIDM[channelID]
	if !ok {
		return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelID)
	}
	return channel, nil
}

func channelIDsForGroupModel(group string, modelName string) []int {
	channelIDs := group2model2channels[group][modelName]
	if len(channelIDs) > 0 {
		return channelIDs
	}
	normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
	if normalizedModel == "" || normalizedModel == modelName {
		return nil
	}
	return group2model2channels[group][normalizedModel]
}

func getChannelExcludingFailedDB(group string, modelName string, failedChannelIDs map[int]struct{}, nextPriorityOnFailure bool) (*Channel, error) {
	abilities, err := retryAbilitiesForGroupModel(group, modelName)
	if err != nil || len(abilities) == 0 {
		return nil, err
	}

	candidates := make([]retrySelectionCandidate, 0, len(abilities))
	for _, ability := range abilities {
		candidates = append(candidates, retrySelectionCandidate{
			channelID: ability.ChannelId,
			priority:  abilityPriority(ability),
			weight:    int(ability.Weight),
		})
	}

	channelID, ok, err := selectRetryCandidateID(candidates, failedChannelIDs, nextPriorityOnFailure)
	if err != nil || !ok {
		return nil, err
	}
	channel := Channel{}
	err = DB.First(&channel, "id = ?", channelID).Error
	return &channel, err
}

func retryAbilitiesForGroupModel(group string, modelName string) ([]Ability, error) {
	abilities, err := findRetryAbilities(group, modelName)
	if err != nil || len(abilities) > 0 {
		return abilities, err
	}
	normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
	if normalizedModel == "" || normalizedModel == modelName {
		return nil, nil
	}
	return findRetryAbilities(group, normalizedModel)
}

func findRetryAbilities(group string, modelName string) ([]Ability, error) {
	var abilities []Ability
	err := DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, modelName, true).
		Order("priority DESC").
		Find(&abilities).Error
	return abilities, err
}

func abilityPriority(ability Ability) int64 {
	if ability.Priority == nil {
		return 0
	}
	return *ability.Priority
}

func selectRetryCandidateID(candidates []retrySelectionCandidate, failedChannelIDs map[int]struct{}, nextPriorityOnFailure bool) (int, bool, error) {
	if len(candidates) == 0 {
		return 0, false, nil
	}
	sort.SliceStable(candidates, func(i int, j int) bool {
		return candidates[i].priority > candidates[j].priority
	})

	var preferredCandidates []retrySelectionCandidate
	if nextPriorityOnFailure {
		// 优先选择"整层都未失败"的最高优先级层
		preferredCandidates = highestFullyUnfailedTierCandidates(candidates, failedChannelIDs)
	}
	if len(preferredCandidates) == 0 {
		// 同优先级模式，或下一优先级模式下已无整层未失败的层：退回到"任意未失败渠道"，
		// 避免重试已知失败的渠道
		preferredCandidates = highestPriorityUnfailedCandidates(candidates, failedChannelIDs)
	}
	if len(preferredCandidates) > 0 {
		id, err := weightedRetryCandidateID(preferredCandidates)
		return id, err == nil, err
	}

	highestPriority := candidates[0].priority
	highestCandidates := make([]retrySelectionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.priority != highestPriority {
			break
		}
		highestCandidates = append(highestCandidates, candidate)
	}
	id, err := weightedRetryCandidateID(highestCandidates)
	return id, err == nil, err
}

// highestPriorityUnfailedCandidates 返回最高优先级中"仍有未失败渠道"的那一层的未失败候选。
// 即：同优先级优先，本层被逐个耗尽后才降级到下一优先级。
func highestPriorityUnfailedCandidates(candidates []retrySelectionCandidate, failedChannelIDs map[int]struct{}) []retrySelectionCandidate {
	for i := 0; i < len(candidates); {
		priority := candidates[i].priority
		samePriority := make([]retrySelectionCandidate, 0)
		for i < len(candidates) && candidates[i].priority == priority {
			if _, failed := failedChannelIDs[candidates[i].channelID]; !failed {
				samePriority = append(samePriority, candidates[i])
			}
			i++
		}
		if len(samePriority) > 0 {
			return samePriority
		}
	}
	return nil
}

// highestFullyUnfailedTierCandidates 返回最高优先级中"整层都未失败"的那一层的全部候选。
// 即：本优先级只要有渠道失败过，就整层跳过、直接尝试下一优先级。
func highestFullyUnfailedTierCandidates(candidates []retrySelectionCandidate, failedChannelIDs map[int]struct{}) []retrySelectionCandidate {
	for i := 0; i < len(candidates); {
		priority := candidates[i].priority
		tier := make([]retrySelectionCandidate, 0)
		tierHasFailure := false
		for i < len(candidates) && candidates[i].priority == priority {
			if _, failed := failedChannelIDs[candidates[i].channelID]; failed {
				tierHasFailure = true
			}
			tier = append(tier, candidates[i])
			i++
		}
		if !tierHasFailure && len(tier) > 0 {
			return tier
		}
	}
	return nil
}

func weightedRetryCandidateID(candidates []retrySelectionCandidate) (int, error) {
	if len(candidates) == 0 {
		return 0, errors.New("channel not found")
	}

	sumWeight := 0
	for _, candidate := range candidates {
		sumWeight += candidate.weight
	}

	smoothingFactor := 1
	smoothingAdjustment := 0
	if sumWeight == 0 {
		sumWeight = len(candidates) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(candidates) < 10 {
		smoothingFactor = 100
	}

	randomWeight := rand.Intn(sumWeight * smoothingFactor)
	for _, candidate := range candidates {
		randomWeight -= candidate.weight*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return candidate.channelID, nil
		}
	}
	return 0, errors.New("channel not found")
}
