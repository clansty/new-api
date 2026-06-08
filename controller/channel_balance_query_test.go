package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
)

func Test_queryOpenAICompatibleBalanceAt_whenUpstreamImplementsNewAPIDashboard(t *testing.T) {
	// Given: a new-api compatible upstream that exposes OpenAI dashboard billing routes.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		switch r.URL.Path {
		case "/v1/dashboard/billing/subscription":
			_, err := w.Write([]byte(`{"has_payment_method":true,"hard_limit_usd":123}`))
			require.NoError(t, err)
		case "/v1/dashboard/billing/usage":
			require.Equal(t, "2026-06-01", r.URL.Query().Get("start_date"))
			require.Equal(t, "2026-06-08", r.URL.Query().Get("end_date"))
			_, err := w.Write([]byte(`{"total_usage":2300}`))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	channel := &model.Channel{
		Key:     "test-key",
		BaseURL: common.GetPointer(server.URL + "/v1"),
	}

	// When: the OpenAI-compatible balance query runs against a /v1 base URL.
	balance, err := queryOpenAICompatibleBalanceAt(channel, time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC))

	// Then: it avoids duplicating /v1 and computes hard limit minus usage.
	require.NoError(t, err)
	require.Equal(t, 100.0, balance)
}

func Test_ensureChannelBalanceBaseURL_whenOpenAIBaseURLIsUnset(t *testing.T) {
	// Given: an OpenAI channel without an explicit BaseURL.
	channel := &model.Channel{
		Type: 1,
	}

	// When: balance update initializes channel defaults before mode dispatch.
	baseURL := ensureChannelBalanceBaseURL(channel)

	// Then: the channel can be queried through the default upstream URL.
	require.Equal(t, "https://api.openai.com", baseURL)
	require.Equal(t, "https://api.openai.com", channel.GetBaseURL())
}

func Test_querySub2APIBalance_whenRemainingIsExplicitZero(t *testing.T) {
	// Given: a sub2api upstream that reports an explicit zero remaining balance.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/usage", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		_, err := w.Write([]byte(`{"remaining":0,"unit":"USD"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	channel := &model.Channel{
		Key:     "test-key",
		BaseURL: common.GetPointer(server.URL),
	}

	// When: the sub2api balance query reads the response.
	balance, err := querySub2APIBalance(channel)

	// Then: the explicit zero is preserved instead of treated as absent.
	require.NoError(t, err)
	require.Equal(t, 0.0, balance)
}

func Test_sub2APIUsageResponseRemainingBalance_whenQuotaRemainingIsPresent(t *testing.T) {
	// Given: a sub2api usage response with the remaining value nested under quota.
	remaining := 42.5
	response := sub2APIUsageResponse{
		Quota: &sub2APIQuota{Remaining: &remaining},
		Unit:  "USD",
	}

	// When: the balance is extracted.
	balance, err := response.remainingBalance()

	// Then: the nested quota value is accepted.
	require.NoError(t, err)
	require.Equal(t, 42.5, balance)
}

func Test_sub2APIUsageResponseRemainingBalance_whenWalletBalanceIsPresent(t *testing.T) {
	// Given: a sub2api unrestricted wallet response with balance as a compatibility alias.
	balanceValue := 28.75
	response := sub2APIUsageResponse{
		Balance: &balanceValue,
		Unit:    "USD",
	}

	// When: the balance is extracted.
	balance, err := response.remainingBalance()

	// Then: the wallet balance is accepted when remaining is absent.
	require.NoError(t, err)
	require.Equal(t, 28.75, balance)
}

func Test_updateChannelBalanceByQueryMode_whenDisabled(t *testing.T) {
	// Given: a channel whose upstream balance query is explicitly disabled.
	channel := &model.Channel{
		Balance: 12.75,
		Setting: common.GetPointer(`{"balance_query_mode":"disabled"}`),
	}

	// When: the query-mode dispatcher is asked to update the balance.
	balance, handled, err := updateChannelBalanceByQueryMode(channel)

	// Then: the mode is handled without making an upstream request or changing the cached balance.
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, 12.75, balance)
}

func Test_shouldSkipChannelBalanceUpdate_whenDisabled(t *testing.T) {
	// Given: a channel with scheduled balance updates disabled.
	channel := &model.Channel{}
	channel.SetSetting(dto.ChannelSettings{BalanceQueryMode: dto.BalanceQueryModeDisabled})

	// When: the scheduled updater checks whether to skip the channel.
	skip := shouldSkipChannelBalanceUpdate(channel)

	// Then: the scheduled updater does not query or auto-disable the channel from stale balance.
	require.True(t, skip)
}
