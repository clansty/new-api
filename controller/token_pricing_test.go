package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestBuildTokenModelPricingDoesNotFlattenTieredPrices(t *testing.T) {
	item := buildTokenModelPricing(model.Pricing{
		ModelName:       "tiered-model",
		QuotaType:       0,
		ModelRatio:      37.5,
		CompletionRatio: 8,
		BillingMode:     "tiered_expr",
		BillingExpr:     `len <= 200000 ? tier("standard", (p) * 1.5 + c * 7.5 + cr * 0.15 + cc * 1.875 + cc1h * 3) : tier("long_context", p * 3 + c * 11.25 + cr * 0.3 + cc * 3.75 + cc1h * 6)`,
	}, 2)

	require.Equal(t, "tiered", item.BillingType)
	require.Nil(t, item.InputPrice)
	require.Nil(t, item.OutputPrice)
	require.Len(t, item.Tiers, 2)
	require.Equal(t, "standard", item.Tiers[0].Label)
	require.Equal(t, "len <= 200000", item.Tiers[0].Condition)
	require.Equal(t, 3.0, *item.Tiers[0].InputPrice)
	require.Equal(t, 15.0, *item.Tiers[0].OutputPrice)
	require.Equal(t, 0.3, *item.Tiers[0].CacheReadPrice)
	require.Equal(t, 3.75, *item.Tiers[0].CacheWritePrice)
	require.Equal(t, 6.0, *item.Tiers[0].CacheWrite1hPrice)
	require.Equal(t, "long_context", item.Tiers[1].Label)
	require.Equal(t, "len > 200000", item.Tiers[1].Condition)
	require.Equal(t, 6.0, *item.Tiers[1].InputPrice)
	require.Equal(t, 22.5, *item.Tiers[1].OutputPrice)
}
