package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type subTokenRequest struct {
	Name string `json:"name"`
}

type subTokenUpdateRequest struct {
	Name   *string `json:"name"`
	Status *int    `json:"status"`
}

func subTokenResponse(token *model.Token) gin.H {
	return gin.H{
		"id":           token.Id,
		"parent_id":    token.ParentId,
		"name":         token.Name,
		"status":       token.Status,
		"key":          "sk-" + token.Key,
		"created_time": token.CreatedTime,
	}
}

func getSubTokenParent(c *gin.Context, parentId int) (*model.Token, bool) {
	userId := c.GetInt("id")
	parent, err := model.GetTokenByIds(parentId, userId)
	if err != nil || parent.ParentId != 0 {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiError(c, err)
		} else if err != nil {
			common.ApiError(c, err)
		} else {
			common.ApiError(c, model.ErrSubTokenParentRequired)
		}
		return nil, false
	}
	return parent, true
}

func listSubTokens(c *gin.Context, parentId int) {
	parent, ok := getSubTokenParent(c, parentId)
	if !ok {
		return
	}
	tokens, err := model.GetSubTokens(parent.Id, parent.UserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]*model.Token, 0, len(tokens))
	for _, token := range tokens {
		items = append(items, model.EffectiveToken(token, parent))
	}
	common.ApiSuccess(c, buildMaskedTokenResponses(items))
}

func createSubToken(c *gin.Context, parentId int) {
	parent, ok := getSubTokenParent(c, parentId)
	if !ok {
		return
	}
	var request subTokenRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	name := strings.TrimSpace(request.Name)
	if name == "" || len(name) > 50 {
		common.ApiError(c, errors.New("子令牌名称长度必须为 1-50 个字符"))
		return
	}
	count, err := model.CountUserTokens(parent.UserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if int(count) >= operation_setting.GetMaxUserTokens() {
		common.ApiError(c, errors.New("已达到最大令牌数量限制"))
		return
	}
	token, err := model.CreateSubToken(parent.Id, parent.UserId, name)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    subTokenResponse(token),
	})
}

func deleteSubToken(c *gin.Context, parentId int, subTokenId int) {
	if _, ok := getSubTokenParent(c, parentId); !ok {
		return
	}
	if err := model.DeleteSubToken(parentId, c.GetInt("id"), subTokenId); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func GetSubTokens(c *gin.Context) {
	parentId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	listSubTokens(c, parentId)
}

func CreateSubToken(c *gin.Context) {
	parentId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	createSubToken(c, parentId)
}

func DeleteSubToken(c *gin.Context) {
	parentId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	subTokenId, err := strconv.Atoi(c.Param("subkey_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	deleteSubToken(c, parentId, subTokenId)
}

func getApiParentToken(c *gin.Context) (*model.Token, bool) {
	token, err := model.GetTokenById(c.GetInt("token_id"))
	if err != nil || token.ParentId != 0 {
		common.ApiError(c, model.ErrSubTokenParentRequired)
		return nil, false
	}
	return token, true
}

func GetSubTokensByApiKey(c *gin.Context) {
	parent, ok := getApiParentToken(c)
	if !ok {
		return
	}
	listSubTokens(c, parent.Id)
}

func CreateSubTokenByApiKey(c *gin.Context) {
	parent, ok := getApiParentToken(c)
	if !ok {
		return
	}
	createSubToken(c, parent.Id)
}

func DeleteSubTokenByApiKey(c *gin.Context) {
	parent, ok := getApiParentToken(c)
	if !ok {
		return
	}
	subTokenId, err := strconv.Atoi(c.Param("subkey_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	deleteSubToken(c, parent.Id, subTokenId)
}

func UpdateSubTokenByApiKey(c *gin.Context) {
	parent, ok := getApiParentToken(c)
	if !ok {
		return
	}
	subTokenId, err := strconv.Atoi(c.Param("subkey_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request subTokenUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	var token model.Token
	if err := model.DB.Where("id = ? AND parent_id = ? AND user_id = ?", subTokenId, parent.Id, parent.UserId).First(&token).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if name == "" || len(name) > 50 {
			common.ApiError(c, errors.New("子令牌名称长度必须为 1-50 个字符"))
			return
		}
		token.Name = name
	}
	if request.Status != nil {
		if *request.Status != common.TokenStatusEnabled && *request.Status != common.TokenStatusDisabled {
			common.ApiError(c, errors.New("子令牌状态无效"))
			return
		}
		token.Status = *request.Status
	}
	if request.Name == nil && request.Status == nil {
		common.ApiError(c, errors.New("至少提供一个要修改的字段"))
		return
	}
	if err := token.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": subTokenResponse(&token)})
}

func GetSubTokenKeyByApiKey(c *gin.Context) {
	parent, ok := getApiParentToken(c)
	if !ok {
		return
	}
	subTokenId, err := strconv.Atoi(c.Param("subkey_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var token model.Token
	if err := model.DB.Where("id = ? AND parent_id = ? AND user_id = ?", subTokenId, parent.Id, parent.UserId).First(&token).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"key": "sk-" + token.GetFullKey()}})
}
