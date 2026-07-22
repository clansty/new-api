package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestCalculateAudioOriginalQuota_whenGroupIsFree(t *testing.T) {
	// Given: 音频请求所在分组免费，模型仍有固定价格。
	info := QuotaInfo{
		ModelName:  "cost-audio",
		UsePrice:   true,
		ModelPrice: 0.25,
		GroupRatio: 0,
	}

	// When: 计算未乘分组倍率的原价。
	originalQuota := calculateAudioOriginalQuota(info)

	// Then: 原价保留固定价格对应的额度。
	require.InDelta(t, 0.25*common.QuotaPerUnit, originalQuota, 0.0001)
}
