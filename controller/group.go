package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetUserGroups(c *gin.Context) {
	userId := c.GetInt("id")
	userGroup, _ := model.GetUserGroup(userId, false)
	usableGroups, groupRequired := buildUserSelectableGroups(userGroup)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
		// group_required 为 true 表示用户所在分组被禁止作为令牌分组，
		// 创建令牌时必须手动选择一个分组，不能留空
		"group_required": groupRequired,
	})
}

func AdminGetUserGroups(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	usableGroups, groupRequired := buildUserSelectableGroups(user.Group)
	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        "",
		"data":           usableGroups,
		"group_required": groupRequired,
	})
}

// buildUserSelectableGroups 返回用户创建令牌时可选的分组列表。
// 第二个返回值为 true 时表示用户所在分组被禁止自选，令牌分组必填。
func buildUserSelectableGroups(userGroup string) (map[string]map[string]interface{}, bool) {
	usableGroups := make(map[string]map[string]interface{})
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	groupRequired := setting.ContainsUserSelfUnusableGroup(userGroup)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			if groupRequired && groupName == userGroup {
				continue
			}
			usableGroups[groupName] = map[string]interface{}{
				"ratio": service.GetUserGroupRatio(userGroup, groupName),
				"desc":  desc,
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]interface{}{
			"ratio": "自动",
			"desc":  setting.GetUsableGroupDescription("auto"),
		}
	}
	return usableGroups, groupRequired
}
