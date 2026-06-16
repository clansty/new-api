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

func GetRandomSatisfiedChannelExcludingFailed(group string, modelName string, failedChannelIDs map[int]struct{}) (*Channel, error) {
	if len(failedChannelIDs) == 0 {
		return GetRandomSatisfiedChannel(group, modelName, 0)
	}
	if !common.MemoryCacheEnabled {
		return getChannelExcludingFailedDB(group, modelName, failedChannelIDs)
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

	channelID, ok, err := selectRetryCandidateID(candidates, failedChannelIDs)
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

func getChannelExcludingFailedDB(group string, modelName string, failedChannelIDs map[int]struct{}) (*Channel, error) {
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

	channelID, ok, err := selectRetryCandidateID(candidates, failedChannelIDs)
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

func selectRetryCandidateID(candidates []retrySelectionCandidate, failedChannelIDs map[int]struct{}) (int, bool, error) {
	if len(candidates) == 0 {
		return 0, false, nil
	}
	sort.SliceStable(candidates, func(i int, j int) bool {
		return candidates[i].priority > candidates[j].priority
	})

	unfailedCandidates := highestPriorityUnfailedCandidates(candidates, failedChannelIDs)
	if len(unfailedCandidates) > 0 {
		id, err := weightedRetryCandidateID(unfailedCandidates)
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
