package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
)

func TestResolveSub2APIAuthUpdate_whenPasswordIsBlank(t *testing.T) {
	// Given: 已配置 sub2api 登录信息的渠道，只修改其他渠道设置。
	origin := &model.Channel{
		Sub2APIUsername:             "user@example.com",
		Sub2APIPassword:             "saved-password",
		Sub2APIAccessToken:          "saved-access",
		Sub2APIRefreshToken:         "saved-refresh",
		Sub2APIAccessTokenExpiresAt: 1_700_000_000,
	}
	incoming := &model.Channel{Sub2APIUsername: "user@example.com"}
	incoming.SetSetting(dto.ChannelSettings{BalanceQueryMode: dto.BalanceQueryModeSub2API})
	emptyPassword := ""

	// When: 更新请求没有重新填写只写密码。
	state, err := resolveSub2APIAuthUpdate(incoming, origin, &emptyPassword)

	// Then: 保留已保存密码和 Token，不要求每次编辑都重新输入。
	require.NoError(t, err)
	require.Equal(t, "saved-password", state.Password)
	require.Equal(t, "saved-access", state.AccessToken)
	require.Equal(t, "saved-refresh", state.RefreshToken)
}

func TestResolveSub2APIAuthUpdate_whenModeChanges(t *testing.T) {
	// Given: 渠道不再使用 sub2api 余额查询模式。
	origin := &model.Channel{
		Sub2APIUsername:     "user@example.com",
		Sub2APIPassword:     "saved-password",
		Sub2APIRefreshToken: "saved-refresh",
	}
	incoming := &model.Channel{Setting: common.GetPointer(`{"balance_query_mode":"openai"}`)}

	// When: 保存渠道设置。
	state, err := resolveSub2APIAuthUpdate(incoming, origin, nil)

	// Then: 不再需要的上游登录凭据和倍率快照被清除。
	require.NoError(t, err)
	require.Empty(t, state.Username)
	require.Empty(t, state.Password)
	require.Nil(t, state.RateMultiplier)
}

func TestPatchChannelJSON_whenPasswordWasSubmitted(t *testing.T) {
	// Given: 编辑请求携带了只写的 sub2api 密码。
	request := PatchChannel{}
	require.NoError(t, common.Unmarshal([]byte(`{"id":12,"name":"sub2api channel","sub2api_password":"password-secret"}`), &request))
	require.Equal(t, 12, request.Id)
	require.Equal(t, "sub2api channel", request.Name)
	require.NotNil(t, request.Sub2APIPassword)
	require.Equal(t, "password-secret", *request.Sub2APIPassword)

	// When: 控制器把更新结果序列化为响应。
	payload, err := common.Marshal(request)

	// Then: 密码字段和值都不会出现在响应中。
	require.NoError(t, err)
	require.NotContains(t, string(payload), "sub2api_password")
	require.NotContains(t, string(payload), "password-secret")
}

func TestPatchChannelJSON_whenManualUpstreamRateIsCleared(t *testing.T) {
	// Given: 编辑请求显式清空手动上游倍率。
	request := PatchChannel{}

	// When: 控制器解析请求。
	err := common.Unmarshal([]byte(`{"id":12,"manual_upstream_rate_multiplier":null}`), &request)

	// Then: 空值和字段存在状态都被保留，数据库更新可据此写入 NULL。
	require.NoError(t, err)
	require.Nil(t, request.ManualUpstreamRateMultiplier)
	require.True(t, request.ManualUpstreamRateMultiplierSet)
}
