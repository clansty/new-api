package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettleTestQuotaUsesTieredBilling(t *testing.T) {
	info := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:   "tiered_expr",
			ExprString:    `param("stream") == true ? tier("stream", p * 3) : tier("base", p * 2)`,
			ExprHash:      billingexpr.ExprHashString(`param("stream") == true ? tier("stream", p * 3) : tier("base", p * 2)`),
			GroupRatio:    1,
			EstimatedTier: "stream",
			QuotaPerUnit:  common.QuotaPerUnit,
			ExprVersion:   1,
		},
		BillingRequestInput: &billingexpr.RequestInput{
			Body: []byte(`{"stream":true}`),
		},
	}

	quota, result := settleTestQuota(info, types.PriceData{
		ModelRatio:      1,
		CompletionRatio: 2,
	}, &dto.Usage{
		PromptTokens: 1000,
	})

	require.Equal(t, 1500, quota)
	require.NotNil(t, result)
	require.Equal(t, "stream", result.MatchedTier)
}

func TestBuildTestLogOtherInjectsTieredInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	info := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode: "tiered_expr",
			ExprString:  `tier("base", p * 2)`,
		},
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	priceData := types.PriceData{
		GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
	}
	usage := &dto.Usage{
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 12,
		},
	}

	other := buildTestLogOther(ctx, info, priceData, usage, &billingexpr.TieredResult{
		MatchedTier: "base",
	})

	require.Equal(t, "tiered_expr", other["billing_mode"])
	require.Equal(t, "base", other["matched_tier"])
	require.NotEmpty(t, other["expr_b64"])
}

func TestShouldTestChannelAutomatically_whenAutoDisabledRecoveryDisabled(t *testing.T) {
	channel := &model.Channel{
		Status:      common.ChannelStatusAutoDisabled,
		AutoRecover: common.GetPointer(0),
	}

	require.False(t, shouldTestChannelAutomatically(channel, channelTestScopeAll))
}

func TestShouldTestChannelAutomatically_whenAutoDisabledRecoveryUnset(t *testing.T) {
	channel := &model.Channel{
		Status: common.ChannelStatusAutoDisabled,
	}

	require.True(t, shouldTestChannelAutomatically(channel, channelTestScopeAll))
	require.True(t, shouldTestChannelAutomatically(channel, channelTestScopeAutoDisabled))
}

func TestShouldTestChannelAutomatically_whenManuallyDisabled(t *testing.T) {
	channel := &model.Channel{
		Status:      common.ChannelStatusManuallyDisabled,
		AutoRecover: common.GetPointer(1),
	}

	require.False(t, shouldTestChannelAutomatically(channel, channelTestScopeAll))
	require.False(t, shouldTestChannelAutomatically(channel, channelTestScopeAutoDisabled))
}

func TestShouldTestChannelAutomatically_whenRecoveryScopeSkipsEnabledChannel(t *testing.T) {
	channel := &model.Channel{
		Status: common.ChannelStatusEnabled,
	}

	require.True(t, shouldTestChannelAutomatically(channel, channelTestScopeAll))
	require.False(t, shouldTestChannelAutomatically(channel, channelTestScopeAutoDisabled))
}
