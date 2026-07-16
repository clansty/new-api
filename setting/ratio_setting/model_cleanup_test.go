package ratio_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestFilterUncoveredModels_removesTieredBillingConfiguration(t *testing.T) {
	billingConfig := config.GlobalConfig.Get("billing_setting")
	require.NotNil(t, billingConfig)
	previousModes, err := common.Marshal(billing_setting.GetBillingModeCopy())
	require.NoError(t, err)
	previousExpressions, err := common.Marshal(billing_setting.GetBillingExprCopy())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, config.UpdateConfigFromMap(billingConfig, map[string]string{
			"billing_mode": string(previousModes),
			"billing_expr": string(previousExpressions),
		}))
	})

	require.NoError(t, config.UpdateConfigFromMap(billingConfig, map[string]string{
		"billing_mode": `{"kept-model":"tiered_expr","missing-model":"tiered_expr"}`,
		"billing_expr": `{"kept-model":"p * 1","missing-model":"p * 2"}`,
	}))

	changed, _, removedModels := FilterUncoveredModels([]string{"kept-model"})

	require.JSONEq(t, `{"kept-model":"tiered_expr"}`, changed["billing_setting.billing_mode"])
	require.JSONEq(t, `{"kept-model":"p * 1"}`, changed["billing_setting.billing_expr"])
	require.Contains(t, removedModels, "missing-model")
}
