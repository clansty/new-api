package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHandleGroupRatioUsesSpecialRatioByDefault(t *testing.T) {
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalGroupGroupRatio := ratio_setting.GroupGroupRatio2JSONString()
	originalModelIgnoreGroupSpecialRatio := ratio_setting.ModelIgnoreGroupSpecialRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatio))
		require.NoError(t, ratio_setting.UpdateModelIgnoreGroupSpecialRatioByJSONString(originalModelIgnoreGroupSpecialRatio))
	})

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"standard":1.25}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"standard":0.5}}`))
	require.NoError(t, ratio_setting.UpdateModelIgnoreGroupSpecialRatioByJSONString(`{}`))

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		OriginModelName: "discounted-model",
		UserGroup:       "vip",
		UsingGroup:      "standard",
	}

	ratioInfo := HandleGroupRatio(ctx, info)

	require.Equal(t, 0.5, ratioInfo.GroupRatio)
	require.Equal(t, 0.5, ratioInfo.GroupSpecialRatio)
	require.True(t, ratioInfo.HasSpecialRatio)
}

func TestHandleGroupRatioUsesBaseRatioWhenModelIgnoresSpecialRatio(t *testing.T) {
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalGroupGroupRatio := ratio_setting.GroupGroupRatio2JSONString()
	originalModelIgnoreGroupSpecialRatio := ratio_setting.ModelIgnoreGroupSpecialRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatio))
		require.NoError(t, ratio_setting.UpdateModelIgnoreGroupSpecialRatioByJSONString(originalModelIgnoreGroupSpecialRatio))
	})

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"standard":1.25}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"standard":0.5}}`))
	require.NoError(t, ratio_setting.UpdateModelIgnoreGroupSpecialRatioByJSONString(`{"full-price-model":true}`))

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		OriginModelName: "full-price-model",
		UserGroup:       "vip",
		UsingGroup:      "standard",
	}

	ratioInfo := HandleGroupRatio(ctx, info)

	require.Equal(t, 1.25, ratioInfo.GroupRatio)
	require.Equal(t, -1.0, ratioInfo.GroupSpecialRatio)
	require.False(t, ratioInfo.HasSpecialRatio)
}

func TestModelPriceHelperTieredUsesPreloadedRequestInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"tiered-test-model":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"tiered-test-model":"param(\"stream\") == true ? tier(\"stream\", p * 3) : tier(\"base\", p * 2)"}`,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	req.Body = nil
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "tiered-test-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
		BillingRequestInput: &billingexpr.RequestInput{
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    []byte(`{"stream":true}`),
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, 1500, priceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.Equal(t, "stream", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, billing_setting.BillingModeTieredExpr, info.TieredBillingSnapshot.BillingMode)
	require.Equal(t, common.QuotaPerUnit, info.TieredBillingSnapshot.QuotaPerUnit)
}
