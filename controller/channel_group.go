package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type channelGroupMemberPayload struct {
	Name    string  `json:"name"`
	Key     string  `json:"key"`
	BaseURL *string `json:"base_url"`
	Proxy   *string `json:"proxy"`
}

func loadChannelGroup(c *gin.Context) (*model.Channel, bool) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	if !channel.IsGroup {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "该渠道不是渠道组"})
		return nil, false
	}
	return channel, true
}

func GetChannelGroupMembers(c *gin.Context) {
	channel, ok := loadChannelGroup(c)
	if !ok {
		return
	}
	members, err := model.GetChannelMembers(channel.Id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, members)
}

func AddChannelGroupMember(c *gin.Context) {
	channel, ok := loadChannelGroup(c)
	if !ok {
		return
	}
	payload := channelGroupMemberPayload{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		common.ApiError(c, err)
		return
	}
	member := model.ChannelMember{
		ChannelId: channel.Id,
		Name:      strings.TrimSpace(payload.Name),
		Key:       strings.TrimSpace(payload.Key),
		BaseURL:   payload.BaseURL,
		Proxy:     payload.Proxy,
	}
	if member.Name == "" || member.Key == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "成员名称和密钥不能为空"})
		return
	}
	if err := model.InsertChannelMember(&member); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	service.ResetProxyClientCache()
	member.Key = ""
	common.ApiSuccess(c, member)
}

func UpdateChannelGroupMember(c *gin.Context) {
	channel, ok := loadChannelGroup(c)
	if !ok {
		return
	}
	memberId, err := strconv.Atoi(c.Param("member_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	payload := channelGroupMemberPayload{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		common.ApiError(c, err)
		return
	}
	member := model.ChannelMember{
		Id:        memberId,
		ChannelId: channel.Id,
		Name:      strings.TrimSpace(payload.Name),
		Key:       strings.TrimSpace(payload.Key),
		BaseURL:   payload.BaseURL,
		Proxy:     payload.Proxy,
	}
	if member.Name == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "成员名称不能为空"})
		return
	}
	if err := model.UpdateChannelMember(&member, member.Key != ""); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	service.ResetProxyClientCache()
	common.ApiSuccess(c, nil)
}

func DeleteChannelGroupMember(c *gin.Context) {
	channel, ok := loadChannelGroup(c)
	if !ok {
		return
	}
	memberId, err := strconv.Atoi(c.Param("member_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteChannelMember(channel.Id, memberId); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	model.InitChannelCache()
	service.ResetProxyClientCache()
	common.ApiSuccess(c, nil)
}

func UpdateChannelGroupMemberStatus(c *gin.Context) {
	channel, ok := loadChannelGroup(c)
	if !ok {
		return
	}
	memberId, err := strconv.Atoi(c.Param("member_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	request := struct {
		Status int `json:"status"`
	}{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if request.Status != common.ChannelStatusEnabled && request.Status != common.ChannelStatusManuallyDisabled {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "成员状态无效"})
		return
	}
	if _, err := model.UpdateChannelMemberStatus(channel.Id, memberId, request.Status, "管理员手动禁用"); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ConvertChannelToGroup(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	request := struct {
		ParallelRequests int `json:"parallel_requests"`
	}{ParallelRequests: 2}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.ConvertChannelToGroup(channelId, request.ParallelRequests); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	model.InitChannelCache()
	service.ResetProxyClientCache()
	common.ApiSuccess(c, gin.H{"id": channelId})
}

func validateChannelGroupMembers(keys []string) ([]model.ChannelMember, error) {
	members := make([]model.ChannelMember, 0, len(keys))
	for i, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		members = append(members, model.ChannelMember{
			Name: fmt.Sprintf("成员 %d", i+1),
			Key:  key,
		})
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("渠道组至少需要一个密钥")
	}
	return members, nil
}
