package model

import (
	"errors"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/sub2apiauth"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Sub2APIAuthCredential struct {
	Key                  string `json:"-" gorm:"type:char(64);primaryKey"`
	AccessToken          string `json:"-" gorm:"type:text"`
	RefreshToken         string `json:"-" gorm:"type:text"`
	AccessTokenExpiresAt int64  `json:"-" gorm:"bigint"`
}

func (Sub2APIAuthCredential) TableName() string {
	return "sub2_api_auth_credentials"
}

func (channel *Channel) LoadSharedSub2APIAuth() (bool, error) {
	if channel.Sub2APIUsername == "" || channel.Sub2APIPassword == "" {
		return false, nil
	}
	credential, found, err := findSub2APIAuthCredential(DB, channel.sub2APIAuthCredentialKey())
	if err != nil || !found {
		return found, err
	}
	channel.Sub2APIAccessToken = credential.AccessToken
	channel.Sub2APIRefreshToken = credential.RefreshToken
	channel.Sub2APIAccessTokenExpiresAt = credential.AccessTokenExpiresAt
	return true, nil
}

func (channel *Channel) SaveSharedSub2APIAuth(state ChannelSub2APIState) error {
	credential := Sub2APIAuthCredential{
		Key:                  channel.sub2APIAuthCredentialKey(),
		AccessToken:          state.AccessToken,
		RefreshToken:         state.RefreshToken,
		AccessTokenExpiresAt: state.AccessTokenExpiresAt,
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := saveSub2APIAuthCredential(tx, credential); err != nil {
			return err
		}
		return tx.Model(channel).Updates(map[string]any{
			"sub2api_access_token":            "",
			"sub2api_refresh_token":           "",
			"sub2api_access_token_expires_at": 0,
		}).Error
	})
	if err != nil {
		return err
	}
	channel.Sub2APIAccessToken = ""
	channel.Sub2APIRefreshToken = ""
	channel.Sub2APIAccessTokenExpiresAt = 0
	return nil
}

func (channel *Channel) sub2APIAuthCredentialKey() string {
	baseURL := ""
	if channel.BaseURL != nil {
		baseURL = *channel.BaseURL
	}
	if baseURL == "" && channel.Type >= 0 && channel.Type < len(constant.ChannelBaseURLs) {
		baseURL = constant.ChannelBaseURLs[channel.Type]
	}
	return sub2apiauth.Identity{
		BaseURL:  baseURL,
		Username: channel.Sub2APIUsername,
		Password: channel.Sub2APIPassword,
	}.Key()
}

func saveSub2APIAuthCredential(db *gorm.DB, credential Sub2APIAuthCredential) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"access_token",
			"refresh_token",
			"access_token_expires_at",
		}),
	}).Create(&credential).Error
}

func migrateSub2APIAuthCredentials() error {
	var channels []Channel
	if err := DB.Where(
		"sub2api_username <> '' AND sub2api_password <> '' AND sub2api_access_token <> '' AND sub2api_refresh_token <> '' AND sub2api_access_token_expires_at > 0",
	).Find(&channels).Error; err != nil {
		return err
	}

	credentials := make(map[string]Sub2APIAuthCredential, len(channels))
	for _, channel := range channels {
		key := channel.sub2APIAuthCredentialKey()
		current := credentials[key]
		if current.AccessTokenExpiresAt >= channel.Sub2APIAccessTokenExpiresAt {
			continue
		}
		credentials[key] = Sub2APIAuthCredential{
			Key:                  key,
			AccessToken:          channel.Sub2APIAccessToken,
			RefreshToken:         channel.Sub2APIRefreshToken,
			AccessTokenExpiresAt: channel.Sub2APIAccessTokenExpiresAt,
		}
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		for _, credential := range credentials {
			stored, found, err := findSub2APIAuthCredential(tx, credential.Key)
			if err != nil {
				return err
			}
			if found && stored.AccessTokenExpiresAt > credential.AccessTokenExpiresAt {
				credential = stored
			}
			if err := saveSub2APIAuthCredential(tx, credential); err != nil {
				return err
			}
		}
		return tx.Model(&Channel{}).Where(
			"sub2api_access_token <> '' OR sub2api_refresh_token <> '' OR sub2api_access_token_expires_at <> 0",
		).Updates(map[string]any{
			"sub2api_access_token":            "",
			"sub2api_refresh_token":           "",
			"sub2api_access_token_expires_at": 0,
		}).Error
	})
}

func findSub2APIAuthCredential(db *gorm.DB, key string) (Sub2APIAuthCredential, bool, error) {
	credential := Sub2APIAuthCredential{Key: key}
	err := db.First(&credential, "key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Sub2APIAuthCredential{}, false, nil
	}
	if err != nil {
		return Sub2APIAuthCredential{}, false, err
	}
	return credential, true, nil
}
