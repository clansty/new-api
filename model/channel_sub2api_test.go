package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestChannelJSON_whenSub2APIAuthIsConfigured(t *testing.T) {
	// Given: 渠道保存了登录凭据、JWT 和倍率快照。
	rate := 0.45
	channel := Channel{
		Sub2APIUsername:             "user@example.com",
		Sub2APIPassword:             "password-secret",
		Sub2APIAccessToken:          "access-secret",
		Sub2APIRefreshToken:         "refresh-secret",
		Sub2APIAccessTokenExpiresAt: 1_700_000_000,
		UpstreamRateMultiplier:      &rate,
		UpstreamGroupName:           "专属 Claude 组",
		UpstreamGroupDescription:    "Claude 专属低倍率分组",
	}

	// When: 渠道通过管理 API 序列化。
	payload, err := common.Marshal(channel)

	// Then: 页面需要的信息可见，所有可用来认证的秘密均不可回传。
	require.NoError(t, err)
	require.Contains(t, string(payload), `"sub2api_username":"user@example.com"`)
	require.Contains(t, string(payload), `"upstream_rate_multiplier":0.45`)
	require.Contains(t, string(payload), `"upstream_group_name":"专属 Claude 组"`)
	require.Contains(t, string(payload), `"upstream_group_description":"Claude 专属低倍率分组"`)
	require.NotContains(t, string(payload), "password-secret")
	require.NotContains(t, string(payload), "access-secret")
	require.NotContains(t, string(payload), "refresh-secret")
}
