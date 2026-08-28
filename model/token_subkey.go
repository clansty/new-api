package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrSubTokenParentRequired = errors.New("母令牌不存在或无效")

func ResolveUserToken(key string) (presented *Token, root *Token, err error) {
	presented, err = GetTokenByKey(key, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrTokenInvalid
		}
		return nil, nil, errors.Join(ErrDatabase, err)
	}
	if presented.Status != common.TokenStatusEnabled {
		return presented, nil, ErrTokenInvalid
	}

	root = presented
	if presented.ParentId == 0 {
		if err := validateRootToken(root); err != nil {
			return presented, nil, err
		}
		return presented, root, nil
	}

	parent, err := GetTokenById(presented.ParentId)
	if err != nil || parent.UserId != presented.UserId || parent.ParentId != 0 {
		return presented, nil, ErrTokenInvalid
	}
	if err := validateRootToken(parent); err != nil {
		return presented, nil, err
	}
	return presented, parent, nil
}

func validateRootToken(token *Token) error {
	if token.Status != common.TokenStatusEnabled {
		return ErrTokenInvalid
	}
	if token.ExpiredTime != -1 && token.ExpiredTime < common.GetTimestamp() {
		if !common.RedisEnabled {
			token.Status = common.TokenStatusExpired
			if err := token.SelectUpdate(); err != nil {
				common.SysLog("failed to update token status" + err.Error())
			}
		}
		return ErrTokenInvalid
	}
	if !token.UnlimitedQuota && token.RemainQuota <= 0 {
		if !common.RedisEnabled {
			token.Status = common.TokenStatusExhausted
			if err := token.SelectUpdate(); err != nil {
				common.SysLog("failed to update token status" + err.Error())
			}
		}
		return ErrTokenInvalid
	}
	return nil
}

func EffectiveToken(presented *Token, root *Token) *Token {
	if presented == nil || root == nil || presented.Id == root.Id {
		return presented
	}
	effective := *presented
	if root.Status != common.TokenStatusEnabled {
		effective.Status = root.Status
	} else if root.ExpiredTime != -1 && root.ExpiredTime < common.GetTimestamp() {
		effective.Status = common.TokenStatusExpired
	} else if !root.UnlimitedQuota && root.RemainQuota <= 0 {
		effective.Status = common.TokenStatusExhausted
	}
	effective.ExpiredTime = root.ExpiredTime
	effective.RemainQuota = root.RemainQuota
	effective.UnlimitedQuota = root.UnlimitedQuota
	effective.ModelLimitsEnabled = root.ModelLimitsEnabled
	effective.ModelLimits = root.ModelLimits
	effective.AllowIps = root.AllowIps
	effective.Group = root.Group
	effective.CrossGroupRetry = root.CrossGroupRetry
	effective.CustomRatio = root.CustomRatio
	effective.UsedQuota = root.UsedQuota
	return &effective
}

func GetSubTokens(parentId int, userId int) ([]*Token, error) {
	if parentId <= 0 || userId <= 0 {
		return nil, ErrSubTokenParentRequired
	}
	var parent Token
	if err := DB.Where("id = ? AND user_id = ? AND parent_id = 0", parentId, userId).First(&parent).Error; err != nil {
		return nil, err
	}
	var tokens []*Token
	err := DB.Where("parent_id = ? AND user_id = ?", parentId, userId).Order("id desc").Find(&tokens).Error
	return tokens, err
}

func CreateSubToken(parentId int, userId int, name string) (*Token, error) {
	if parentId <= 0 || userId <= 0 {
		return nil, ErrSubTokenParentRequired
	}
	var parent Token
	if err := DB.Where("id = ? AND user_id = ? AND parent_id = 0", parentId, userId).First(&parent).Error; err != nil {
		return nil, err
	}
	if parent.Status == common.TokenStatusDisabled || parent.Status == common.TokenStatusExpired ||
		(parent.ExpiredTime != -1 && parent.ExpiredTime < common.GetTimestamp()) {
		return nil, ErrTokenInvalid
	}
	key, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	token := &Token{
		UserId:       userId,
		ParentId:     parentId,
		Key:          key,
		Name:         name,
		Status:       common.TokenStatusEnabled,
		CreatedTime:  now,
		AccessedTime: now,
		ExpiredTime:  -1,
	}
	if err := DB.Create(token).Error; err != nil {
		return nil, err
	}
	return token, nil
}

func DeleteSubToken(parentId int, userId int, subTokenId int) error {
	if parentId <= 0 || userId <= 0 || subTokenId <= 0 {
		return ErrSubTokenParentRequired
	}
	var token Token
	if err := DB.Where("id = ? AND parent_id = ? AND user_id = ?", subTokenId, parentId, userId).First(&token).Error; err != nil {
		return err
	}
	return token.Delete()
}
