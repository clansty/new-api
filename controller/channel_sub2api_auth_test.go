package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

func TestPatchChannelJSON_tracksSettingPresence(t *testing.T) {
	withoutSetting := PatchChannel{}
	withSetting := PatchChannel{}
	withNullSetting := PatchChannel{}

	require.NoError(t, common.Unmarshal([]byte(`{"id":12,"status":2}`), &withoutSetting))
	require.NoError(t, common.Unmarshal([]byte(`{"id":12,"setting":"{\"balance_query_mode\":\"openai\"}"}`), &withSetting))
	require.NoError(t, common.Unmarshal([]byte(`{"id":12,"setting":null}`), &withNullSetting))

	require.False(t, withoutSetting.SettingSet)
	require.True(t, withSetting.SettingSet)
	require.True(t, withNullSetting.SettingSet)
}

func TestResolvePatchSub2APIState_whenSettingIsOmitted(t *testing.T) {
	declaredRate := 0.675
	loginRate := 0.45
	origin := &model.Channel{
		Sub2APIUsername:                "user@example.com",
		Sub2APIPassword:                "saved-password",
		UpstreamRateMultiplier:         &declaredRate,
		UpstreamDeclaredRateMultiplier: &declaredRate,
		UpstreamLoginRateMultiplier:    &loginRate,
		UpstreamGroupName:              "专属 Claude 组",
	}
	incoming := PatchChannel{}
	require.NoError(t, common.Unmarshal([]byte(`{"id":12,"status":2}`), &incoming))

	state, err := resolvePatchSub2APIState(&incoming, origin)

	require.NoError(t, err)
	require.Equal(t, "saved-password", state.Password)
	require.Same(t, origin.UpstreamRateMultiplier, state.RateMultiplier)
	require.Same(t, origin.UpstreamDeclaredRateMultiplier, state.DeclaredRateMultiplier)
	require.Same(t, origin.UpstreamLoginRateMultiplier, state.LoginRateMultiplier)
	require.Equal(t, "专属 Claude 组", state.GroupName)
}

func TestUpdateChannel_whenStatusOnlyPatchPreservesSub2APIRates(t *testing.T) {
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/channel.db"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))

	declaredRate := 0.675
	loginRate := 0.45
	setting := common.GetPointer(`{"balance_query_mode":"sub2api"}`)
	channel := model.Channel{
		Name:                           "sub2api channel",
		Key:                            "sk-upstream",
		Models:                         "gpt-4o",
		Group:                          "default",
		Setting:                        setting,
		Status:                         common.ChannelStatusEnabled,
		Sub2APIUsername:                "user@example.com",
		Sub2APIPassword:                "saved-password",
		UpstreamRateMultiplier:         &declaredRate,
		UpstreamDeclaredRateMultiplier: &declaredRate,
		UpstreamLoginRateMultiplier:    &loginRate,
		UpstreamGroupName:              "专属 Claude 组",
	}
	require.NoError(t, db.Create(&channel).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/channel/", strings.NewReader(fmt.Sprintf(`{"id":%d,"status":2}`, channel.Id)))
	ctx.Request.Header.Set("Content-Type", "application/json")

	UpdateChannel(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var updated model.Channel
	require.NoError(t, db.First(&updated, channel.Id).Error)
	require.Equal(t, common.ChannelStatusManuallyDisabled, updated.Status)
	require.NotNil(t, updated.UpstreamRateMultiplier)
	require.Equal(t, declaredRate, *updated.UpstreamRateMultiplier)
	require.NotNil(t, updated.UpstreamDeclaredRateMultiplier)
	require.Equal(t, declaredRate, *updated.UpstreamDeclaredRateMultiplier)
	require.NotNil(t, updated.UpstreamLoginRateMultiplier)
	require.Equal(t, loginRate, *updated.UpstreamLoginRateMultiplier)
	require.Equal(t, "专属 Claude 组", updated.UpstreamGroupName)
}
