package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetChannelGroups_excludesGroupsMarkedSelfUnusable(t *testing.T) {
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalSelfUnusableGroups := setting.UserSelfUnusableGroups2JsonString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, setting.UpdateUserSelfUnusableGroupsByJsonString(originalSelfUnusableGroups))
	})

	// 给定：staff 仅用于标识用户所属分组，不能配置渠道。
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"staff":1}`))
	require.NoError(t, setting.UpdateUserSelfUnusableGroupsByJsonString(`["staff"]`))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	// 执行
	GetChannelGroups(ctx)

	// 验证
	require.Equal(t, 200, recorder.Code)
	var response struct {
		Success bool     `json:"success"`
		Data    []string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.ElementsMatch(t, []string{"default"}, response.Data)
}
