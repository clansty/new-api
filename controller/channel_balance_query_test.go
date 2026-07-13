package controller

import (
	"context"
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

func Test_querySub2APIChannelSnapshot_whenCredentialsAreConfigured(t *testing.T) {
	// Given: sub2api 同时提供 API Key 余额和登录用户的专属倍率。
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			_, err := w.Write([]byte(`{"remaining":88.5,"unit":"USD"}`))
			require.NoError(t, err)
		case "/api/v1/auth/login":
			_, err := w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"access-1","refresh_token":"refresh-1","expires_in":3600}}`))
			require.NoError(t, err)
		case "/api/v1/keys":
			_, err := w.Write([]byte(`{"code":0,"message":"success","data":{"items":[{"key":"sk-upstream","group":{"id":7,"name":"专属 Claude 组","description":"Claude 专属低倍率分组","rate_multiplier":1.2}}]}}`))
			require.NoError(t, err)
		case "/api/v1/groups/rates":
			_, err := w.Write([]byte(`{"code":0,"message":"success","data":{"7":0.45}}`))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	channel := &model.Channel{
		Key:             "sk-upstream",
		BaseURL:         common.GetPointer(server.URL),
		Sub2APIUsername: "user@example.com",
		Sub2APIPassword: "secret",
	}

	// When: 余额查询模式获取完整的上游快照。
	snapshot, err := querySub2APIChannelSnapshot(context.Background(), channel)

	// Then: 余额、实际倍率、分组名和新 Token 在一次查询中返回。
	require.NoError(t, err)
	require.Equal(t, 88.5, snapshot.Balance)
	require.NotNil(t, snapshot.RateMultiplier)
	require.Equal(t, 0.45, *snapshot.RateMultiplier)
	require.Equal(t, "专属 Claude 组", snapshot.GroupName)
	require.Equal(t, "Claude 专属低倍率分组", snapshot.GroupDescription)
	require.Equal(t, "refresh-1", snapshot.Auth.RefreshToken)
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

func Test_queryHYL2APIBalance_whenBaseURLIncludesV1(t *testing.T) {
	// Given: an hyl2api upstream whose quota endpoint is rooted outside /v1.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/user/api/quota", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		_, err := w.Write([]byte(`{"success":true,"limit_mode":"usd","usd":{"remaining":4102.476372}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	channel := &model.Channel{
		Key:     "test-key",
		BaseURL: common.GetPointer(server.URL + "/v1"),
	}

	// When: the hyl2api balance query reads the quota endpoint.
	balance, err := queryHYL2APIBalance(channel)

	// Then: it strips the OpenAI /v1 suffix and returns the USD remaining balance.
	require.NoError(t, err)
	require.Equal(t, 4102.476372, balance)
}

func Test_hyl2APIQuotaResponseRemainingBalance_whenUSDRemainingIsExplicitZero(t *testing.T) {
	// Given: an hyl2api quota response with an explicit zero USD remaining balance.
	success := true
	remaining := 0.0
	response := hyl2APIQuotaResponse{
		Success: &success,
		USD: &hyl2APIQuotaBucket{
			Remaining: &remaining,
		},
	}

	// When: the balance is extracted.
	balance, err := response.remainingBalance()

	// Then: the explicit zero is preserved instead of treated as absent.
	require.NoError(t, err)
	require.Equal(t, 0.0, balance)
}

func Test_hyl2APIQuotaResponseRemainingBalance_whenResponseIsUnsuccessful(t *testing.T) {
	// Given: an hyl2api quota response that rejected the API key.
	success := false
	response := hyl2APIQuotaResponse{
		Success: &success,
		Detail:  "请提供 API Key",
	}

	// When: the balance is extracted.
	balance, err := response.remainingBalance()

	// Then: the API error is surfaced and no balance is returned.
	require.ErrorContains(t, err, "请提供 API Key")
	require.Equal(t, 0.0, balance)
}

func Test_updateChannelBalanceByQueryMode_whenDisabled(t *testing.T) {
	// Given: a channel whose upstream balance query is explicitly disabled.
	channel := &model.Channel{
		Balance: 12.75,
		Setting: common.GetPointer(`{"balance_query_mode":"disabled"}`),
	}

	// When: the query-mode dispatcher is asked to update the balance.
	balance, handled, err := updateChannelBalanceByQueryMode(context.Background(), channel)

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
