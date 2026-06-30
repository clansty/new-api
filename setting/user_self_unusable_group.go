package setting

import (
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// userSelfUnusableGroups 保存被禁止作为「用户自身分组」令牌分组的分组名集合。
// 当用户所在分组在该集合中时：创建令牌不能不选分组（不会自动落到用户分组），
// 且该分组不会出现在用户创建令牌的下拉框里，用户必须手动选择其他分组。
var userSelfUnusableGroups = make(map[string]bool)
var userSelfUnusableGroupsMutex sync.RWMutex

// ContainsUserSelfUnusableGroup 判断某个分组是否被禁止作为其成员的令牌分组
func ContainsUserSelfUnusableGroup(group string) bool {
	if group == "" {
		return false
	}
	userSelfUnusableGroupsMutex.RLock()
	defer userSelfUnusableGroupsMutex.RUnlock()
	return userSelfUnusableGroups[group]
}

func GetUserSelfUnusableGroups() []string {
	userSelfUnusableGroupsMutex.RLock()
	defer userSelfUnusableGroupsMutex.RUnlock()
	groups := make([]string, 0, len(userSelfUnusableGroups))
	for group := range userSelfUnusableGroups {
		groups = append(groups, group)
	}
	return groups
}

func UpdateUserSelfUnusableGroupsByJsonString(jsonStr string) error {
	groups := make([]string, 0)
	if jsonStr != "" {
		if err := common.Unmarshal([]byte(jsonStr), &groups); err != nil {
			return err
		}
	}
	userSelfUnusableGroupsMutex.Lock()
	defer userSelfUnusableGroupsMutex.Unlock()
	userSelfUnusableGroups = make(map[string]bool, len(groups))
	for _, group := range groups {
		if group == "" {
			continue
		}
		userSelfUnusableGroups[group] = true
	}
	return nil
}

func UserSelfUnusableGroups2JsonString() string {
	groups := GetUserSelfUnusableGroups()
	jsonBytes, err := common.Marshal(groups)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}
