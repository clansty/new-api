package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func Test_migrateSub2APIAuthCredentials_deduplicatesLegacyChannelTokens(t *testing.T) {
	// Given: 同一上游账号在两条渠道中保存了不同版本的 Token。
	truncateTables(t)
	channels := []Channel{
		{
			Name:                        "first",
			Key:                         "sk-first",
			BaseURL:                     common.GetPointer("https://sub2api.example.com"),
			Sub2APIUsername:             "user@example.com",
			Sub2APIPassword:             "secret",
			Sub2APIAccessToken:          "old-access",
			Sub2APIRefreshToken:         "old-refresh",
			Sub2APIAccessTokenExpiresAt: 100,
		},
		{
			Name:                        "second",
			Key:                         "sk-second",
			BaseURL:                     common.GetPointer("https://sub2api.example.com/v1"),
			Sub2APIUsername:             "user@example.com",
			Sub2APIPassword:             "secret",
			Sub2APIAccessToken:          "new-access",
			Sub2APIRefreshToken:         "new-refresh",
			Sub2APIAccessTokenExpiresAt: 200,
		},
	}
	require.NoError(t, DB.Create(&channels).Error)

	// When: 启动迁移汇总旧渠道凭据。
	require.NoError(t, migrateSub2APIAuthCredentials())

	// Then: 只保留最新共享 Token，渠道表中的副本全部清空。
	var credentials []Sub2APIAuthCredential
	require.NoError(t, DB.Find(&credentials).Error)
	require.Len(t, credentials, 1)
	require.Equal(t, "new-access", credentials[0].AccessToken)
	require.Equal(t, "new-refresh", credentials[0].RefreshToken)
	require.Equal(t, int64(200), credentials[0].AccessTokenExpiresAt)

	var storedChannels []Channel
	require.NoError(t, DB.Order("id").Find(&storedChannels).Error)
	require.Len(t, storedChannels, 2)
	for _, channel := range storedChannels {
		require.Empty(t, channel.Sub2APIAccessToken)
		require.Empty(t, channel.Sub2APIRefreshToken)
		require.Zero(t, channel.Sub2APIAccessTokenExpiresAt)
	}
}
