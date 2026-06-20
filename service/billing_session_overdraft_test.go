package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBillingSession_AllowsWalletOverdraftWhenUserAllowsOverdraft(t *testing.T) {
	truncate(t)

	const userID = 101
	const tokenID = 101
	const preConsumed = 3000
	const tokenRemain = 10000

	user := &model.User{
		Id:             userID,
		Username:       "overdraft_user",
		Quota:          0,
		Status:         1,
		AllowOverdraft: true,
	}
	require.NoError(t, model.DB.Create(user).Error)
	seedToken(t, tokenID, userID, "overdraft-token", tokenRemain)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("token_quota", tokenRemain)

	relayInfo := &relaycommon.RelayInfo{
		UserId:          userID,
		UserQuota:       0,
		AllowOverdraft:  true,
		TokenId:         tokenID,
		TokenKey:        "overdraft-token",
		TokenUnlimited:  false,
		OriginModelName: "test-model",
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, preConsumed)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, -preConsumed, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain-preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, relayInfo.FinalPreConsumedQuota)
	assert.Equal(t, BillingSourceWallet, relayInfo.BillingSource)
}

func TestNewBillingSession_BlocksWalletOverdraftByDefault(t *testing.T) {
	truncate(t)

	const userID = 103
	const tokenID = 103
	const preConsumed = 3000
	const tokenRemain = 10000

	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "default-overdraft-token", tokenRemain)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("token_quota", tokenRemain)

	relayInfo := &relaycommon.RelayInfo{
		UserId:          userID,
		UserQuota:       0,
		AllowOverdraft:  false,
		TokenId:         tokenID,
		TokenKey:        "default-overdraft-token",
		TokenUnlimited:  false,
		OriginModelName: "test-model",
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, preConsumed)

	require.Nil(t, session)
	require.NotNil(t, apiErr)
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, 0, getUserQuota(t, userID))
}

func TestNewBillingSession_OverdraftStillRequiresTokenQuota(t *testing.T) {
	truncate(t)

	const userID = 102
	const tokenID = 102
	const preConsumed = 3000
	const tokenRemain = 1000

	user := &model.User{
		Id:             userID,
		Username:       "overdraft_token_limited_user",
		Quota:          0,
		Status:         1,
		AllowOverdraft: true,
	}
	require.NoError(t, model.DB.Create(user).Error)
	seedToken(t, tokenID, userID, "overdraft-limited-token", tokenRemain)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("token_quota", tokenRemain)

	relayInfo := &relaycommon.RelayInfo{
		UserId:          userID,
		UserQuota:       0,
		AllowOverdraft:  true,
		TokenId:         tokenID,
		TokenKey:        "overdraft-limited-token",
		TokenUnlimited:  false,
		OriginModelName: "test-model",
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, preConsumed)

	require.Nil(t, session)
	require.NotNil(t, apiErr)
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, 0, getUserQuota(t, userID))
}
