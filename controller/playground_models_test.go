package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func decodeUserModelsData(t *testing.T, recorder *httptest.ResponseRecorder) []string {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool     `json:"success"`
		Message string   `json:"message"`
		Data    []string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success, payload.Message)
	return payload.Data
}

func newPlaygroundModelsContext(method string, target string, userId int, role int) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, nil)
	ctx.Set("id", userId)
	ctx.Set("role", role)
	return ctx, recorder
}

func TestGetUserModelsFiltersByGroup(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	require.NoError(t, db.Create(&model.User{
		Id:       2001,
		Username: "playground-group-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "pg-default-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "pg-disabled-model", ChannelId: 1, Enabled: false},
		{Group: "other", Model: "pg-other-model", ChannelId: 1, Enabled: true},
	}).Error)

	ctx, recorder := newPlaygroundModelsContext(http.MethodGet, "/api/user/models?group=default", 2001, common.RoleCommonUser)
	GetUserModels(ctx)

	models := decodeUserModelsData(t, recorder)
	require.Contains(t, models, "pg-default-model")
	require.NotContains(t, models, "pg-disabled-model")
	require.NotContains(t, models, "pg-other-model")
}

func TestGetUserModelsRejectsUnauthorizedGroup(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	require.NoError(t, db.Create(&model.User{
		Id:       2002,
		Username: "playground-group-denied-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)

	ctx, recorder := newPlaygroundModelsContext(http.MethodGet, "/api/user/models?group=not-usable", 2002, common.RoleCommonUser)
	GetUserModels(ctx)

	var payload struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.False(t, payload.Success)
}

func TestGetUserModelsByChannelAllowsAdminAndDisabledChannel(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	require.NoError(t, db.Create(&model.User{
		Id:       2003,
		Username: "playground-channel-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:     9001,
		Name:   "playground-disabled-channel",
		Status: common.ChannelStatusManuallyDisabled,
		Models: "pg-channel-model-a, pg-channel-model-b ,pg-channel-model-a",
	}).Error)

	// 普通用户不允许指定渠道
	ctx, recorder := newPlaygroundModelsContext(http.MethodGet, "/api/user/models?channel_id=9001", 2003, common.RoleCommonUser)
	GetUserModels(ctx)

	var denied struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &denied))
	require.False(t, denied.Success)

	// 管理员可以查看禁用渠道声明的模型
	ctx, recorder = newPlaygroundModelsContext(http.MethodGet, "/api/user/models?channel_id=9001", 2003, common.RoleAdminUser)
	GetUserModels(ctx)

	models := decodeUserModelsData(t, recorder)
	require.ElementsMatch(t, []string{"pg-channel-model-a", "pg-channel-model-b"}, models)
}

func TestGetPlaygroundChannelsIncludesDisabledChannels(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	require.NoError(t, db.Create(&[]model.Channel{
		{Id: 9101, Name: "pg-enabled-channel", Status: common.ChannelStatusEnabled, Models: "m1"},
		{Id: 9102, Name: "pg-disabled-channel", Status: common.ChannelStatusManuallyDisabled, Models: "m2"},
	}).Error)

	ctx, recorder := newPlaygroundModelsContext(http.MethodGet, "/api/channel/playground", 2004, common.RoleAdminUser)
	GetPlaygroundChannels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool                      `json:"success"`
		Data    []PlaygroundChannelOption `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	byId := make(map[int]PlaygroundChannelOption, len(payload.Data))
	for _, option := range payload.Data {
		byId[option.Id] = option
	}
	require.Equal(t, "pg-enabled-channel", byId[9101].Name)
	require.Equal(t, common.ChannelStatusEnabled, byId[9101].Status)
	require.Equal(t, "pg-disabled-channel", byId[9102].Name)
	require.Equal(t, common.ChannelStatusManuallyDisabled, byId[9102].Status)
}
