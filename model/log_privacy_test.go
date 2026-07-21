package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestFormatUserLogs_HidesModelMappingDetails_whenLogUsesMappedModel(t *testing.T) {
	// Given: 映射详情只允许管理员用于诊断，普通计费详情仍需保留。
	logs := []*Log{{
		Other: `{"is_model_mapped":true,"upstream_model_name":"actual-model","model_ratio":2}`,
	}}

	// When: 日志经过普通用户响应格式化。
	formatUserLogs(logs, 0)

	// Then: 响应与未映射日志一致，且不破坏普通计费详情。
	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.NotContains(t, other, "is_model_mapped")
	require.NotContains(t, other, "upstream_model_name")
	require.Equal(t, float64(2), other["model_ratio"])
}

func TestGetAllLogs_PreservesModelMappingDetails_forAdminQuery(t *testing.T) {
	// Given: 管理员需要映射详情诊断实际转发模型。
	truncateTables(t)
	require.NoError(t, LOG_DB.Create(&Log{
		Type:      LogTypeConsume,
		ModelName: "requested-model",
		Other:     `{"is_model_mapped":true,"upstream_model_name":"actual-model"}`,
	}).Error)

	// When: 通过管理员日志查询读取记录。
	logs, total, err := GetAllLogs(LogTypeUnknown, 0, 0, "", "", "", 0, 10, 0, "", "")

	// Then: 管理员仍能看到请求模型与实际模型的映射关系。
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Equal(t, true, other["is_model_mapped"])
	require.Equal(t, "actual-model", other["upstream_model_name"])
}
