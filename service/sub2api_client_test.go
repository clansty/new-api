package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestSub2APIClientQueryMetadata_whenUserHasCustomGroupRate(t *testing.T) {
	// Given: 上游需要登录，并给当前用户配置了专属分组倍率。
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/login":
			var payload struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			require.Equal(t, "user@example.com", payload.Email)
			require.Equal(t, "secret", payload.Password)
			_, err := w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"access-1","refresh_token":"refresh-1","expires_in":3600}}`))
			require.NoError(t, err)
		case "/api/v1/keys":
			require.Equal(t, "Bearer access-1", r.Header.Get("Authorization"))
			require.Equal(t, "sk-upstream", r.URL.Query().Get("search"))
			_, err := w.Write([]byte(`{"code":0,"message":"success","data":{"items":[{"key":"sk-upstream","group":{"id":7,"name":"专属 Claude 组","description":"Claude 专属低倍率分组","rate_multiplier":1.2}}]}}`))
			require.NoError(t, err)
		case "/api/v1/groups/rates":
			require.Equal(t, "Bearer access-1", r.Header.Get("Authorization"))
			_, err := w.Write([]byte(`{"code":0,"message":"success","data":{"7":0.45}}`))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewSub2APIClient(server.URL, "")
	require.NoError(t, err)
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	auth := Sub2APIAuthState{Email: "user@example.com", Password: "secret"}

	// When: new-api 查询该渠道 API Key 对应的实际倍率。
	metadata, updatedAuth, err := client.QueryMetadata(context.Background(), "sk-upstream", auth)

	// Then: 专属倍率覆盖分组默认倍率，同时返回可持久化的 Token。
	require.NoError(t, err)
	require.Equal(t, 0.45, metadata.RateMultiplier)
	require.Equal(t, "专属 Claude 组", metadata.GroupName)
	require.Equal(t, "Claude 专属低倍率分组", metadata.GroupDescription)
	require.Equal(t, "access-1", updatedAuth.AccessToken)
	require.Equal(t, "refresh-1", updatedAuth.RefreshToken)
	require.Equal(t, int64(1_700_003_600), updatedAuth.AccessTokenExpiresAt)
}

func TestSub2APIClientQueryMetadata_whenRefreshTokenRotates(t *testing.T) {
	// Given: Access Token 已过期，但 Refresh Token 仍有效且上游会轮换它。
	loginCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/login":
			loginCalled = true
			http.Error(w, "unexpected login", http.StatusInternalServerError)
		case "/api/v1/auth/refresh":
			_, err := w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"access-2","refresh_token":"refresh-2","expires_in":1800}}`))
			require.NoError(t, err)
		case "/api/v1/keys":
			require.Equal(t, "Bearer access-2", r.Header.Get("Authorization"))
			_, err := w.Write([]byte(`{"code":0,"message":"success","data":{"items":[{"key":"sk-upstream","group":{"id":9,"name":"OpenAI 组","rate_multiplier":0.8}}]}}`))
			require.NoError(t, err)
		case "/api/v1/groups/rates":
			_, err := w.Write([]byte(`{"code":0,"message":"success","data":{}}`))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewSub2APIClient(server.URL, "")
	require.NoError(t, err)
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	auth := Sub2APIAuthState{
		Email:                "user@example.com",
		Password:             "secret",
		AccessToken:          "expired-access",
		RefreshToken:         "refresh-1",
		AccessTokenExpiresAt: 1_699_999_999,
	}

	// When: 查询实际倍率。
	metadata, updatedAuth, err := client.QueryMetadata(context.Background(), "sk-upstream", auth)

	// Then: 使用轮换后的 Token，且无需再次提交密码。
	require.NoError(t, err)
	require.False(t, loginCalled)
	require.Equal(t, 0.8, metadata.RateMultiplier)
	require.Equal(t, "refresh-2", updatedAuth.RefreshToken)
	require.Equal(t, int64(1_700_001_800), updatedAuth.AccessTokenExpiresAt)
}

func TestSub2APIClientQueryMetadata_reusesAuthForSameCredentials(t *testing.T) {
	// Given: 两个渠道指向同一上游，并使用相同的登录账号。
	var loginCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/login":
			loginCalls.Add(1)
			_, err := w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"shared-access","refresh_token":"shared-refresh","expires_in":3600}}`))
			require.NoError(t, err)
		case "/api/v1/keys":
			apiKey := r.URL.Query().Get("search")
			_, err := w.Write([]byte(`{"code":0,"message":"success","data":{"items":[{"key":"` + apiKey + `","group":{"id":7,"name":"共享分组","rate_multiplier":0.8}}]}}`))
			require.NoError(t, err)
		case "/api/v1/groups/rates":
			_, err := w.Write([]byte(`{"code":0,"message":"success","data":{}}`))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	firstClient, err := NewSub2APIClient(server.URL, "")
	require.NoError(t, err)
	secondClient, err := NewSub2APIClient(server.URL+"/v1", "")
	require.NoError(t, err)
	auth := Sub2APIAuthState{Email: "user@example.com", Password: "secret"}

	// When: 两条渠道并发查询各自 API Key 的分组。
	start := make(chan struct{})
	queryErrors := make(chan error, 2)
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	for index, client := range []*Sub2APIClient{firstClient, secondClient} {
		go func() {
			defer waitGroup.Done()
			<-start
			_, _, queryErr := client.QueryMetadata(context.Background(), "sk-"+[]string{"first", "second"}[index], auth)
			queryErrors <- queryErr
		}()
	}
	close(start)
	waitGroup.Wait()
	close(queryErrors)

	// Then: 规范化后相同的上游账号只登录一次。
	for queryErr := range queryErrors {
		require.NoError(t, queryErr)
	}
	require.Equal(t, int32(1), loginCalls.Load())
}
