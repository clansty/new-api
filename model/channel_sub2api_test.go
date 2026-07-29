package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestChannelJSON_whenSub2APIAuthIsConfigured(t *testing.T) {
	// Given: 渠道保存了登录凭据、JWT 和倍率快照。
	rate := 0.45
	declaredRate := 0.675
	channel := Channel{
		Sub2APIUsername:                "user@example.com",
		Sub2APIPassword:                "password-secret",
		Sub2APIAccessToken:             "access-secret",
		Sub2APIRefreshToken:            "refresh-secret",
		Sub2APIAccessTokenExpiresAt:    1_700_000_000,
		UpstreamRateMultiplier:         &rate,
		UpstreamDeclaredRateMultiplier: &declaredRate,
		UpstreamLoginRateMultiplier:    &rate,
		UpstreamGroupName:              "专属 Claude 组",
		UpstreamGroupDescription:       "Claude 专属低倍率分组",
	}

	// When: 渠道通过管理 API 序列化。
	payload, err := common.Marshal(channel)

	// Then: 页面需要的信息可见，所有可用来认证的秘密均不可回传。
	require.NoError(t, err)
	require.Contains(t, string(payload), `"sub2api_username":"user@example.com"`)
	require.Contains(t, string(payload), `"upstream_rate_multiplier":0.45`)
	require.Contains(t, string(payload), `"upstream_declared_rate_multiplier":0.675`)
	require.Contains(t, string(payload), `"upstream_login_rate_multiplier":0.45`)
	require.Contains(t, string(payload), `"upstream_group_name":"专属 Claude 组"`)
	require.Contains(t, string(payload), `"upstream_group_description":"Claude 专属低倍率分组"`)
	require.NotContains(t, string(payload), "password-secret")
	require.NotContains(t, string(payload), "access-secret")
	require.NotContains(t, string(payload), "refresh-secret")
}

func TestChannelEffectiveUpstreamRateMultiplier_whenAutomaticRateIsMissing(t *testing.T) {
	// Given: 渠道仅配置了手动上游倍率。
	manualRate := 0.6
	channel := Channel{ManualUpstreamRateMultiplier: &manualRate}

	// When: 读取成本计算使用的有效上游倍率。
	rate, manual := channel.EffectiveUpstreamRateMultiplier()

	// Then: 使用手动倍率并标记来源。
	require.NotNil(t, rate)
	require.Equal(t, 0.6, *rate)
	require.True(t, manual)
}

func TestChannelEffectiveUpstreamRateMultiplier_whenAutomaticRateExists(t *testing.T) {
	// Given: 渠道同时保留了自动倍率和手动兜底倍率。
	automaticRate := 0.45
	manualRate := 0.6
	channel := Channel{
		UpstreamRateMultiplier:       &automaticRate,
		ManualUpstreamRateMultiplier: &manualRate,
	}

	// When: 读取成本计算使用的有效上游倍率。
	rate, manual := channel.EffectiveUpstreamRateMultiplier()

	// Then: 自动倍率优先，手动倍率仅作为兜底。
	require.NotNil(t, rate)
	require.Equal(t, 0.45, *rate)
	require.False(t, manual)
}
