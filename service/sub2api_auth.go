package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type sub2APITokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Requires2FA  bool   `json:"requires_2fa"`
}

type sub2APILoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type sub2APIRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (client *Sub2APIClient) ensureAccessToken(ctx context.Context, auth Sub2APIAuthState) (Sub2APIAuthState, error) {
	if auth.AccessToken != "" && auth.AccessTokenExpiresAt > client.now().Add(time.Minute).Unix() {
		return auth, nil
	}
	if auth.RefreshToken != "" {
		refreshed, err := client.refreshToken(ctx, auth)
		if err == nil {
			return refreshed, nil
		}
	}
	return client.login(ctx, auth)
}

func (client *Sub2APIClient) login(ctx context.Context, auth Sub2APIAuthState) (Sub2APIAuthState, error) {
	if strings.TrimSpace(auth.Email) == "" || auth.Password == "" {
		return auth, errors.New("sub2api username and password are required")
	}
	payload, err := common.Marshal(sub2APILoginRequest{Email: auth.Email, Password: auth.Password})
	if err != nil {
		return auth, err
	}
	envelope, err := requestSub2API[sub2APITokenData](ctx, client.httpClient, sub2APIRequest{
		method: http.MethodPost,
		url:    client.rootURL + "/api/v1/auth/login",
		body:   payload,
	})
	if err != nil {
		return auth, fmt.Errorf("login to sub2api: %w", err)
	}
	if envelope.Data.Requires2FA {
		return auth, ErrSub2API2FARequired
	}
	return client.applyTokenData(auth, envelope.Data)
}

func (client *Sub2APIClient) refreshToken(ctx context.Context, auth Sub2APIAuthState) (Sub2APIAuthState, error) {
	payload, err := common.Marshal(sub2APIRefreshRequest{RefreshToken: auth.RefreshToken})
	if err != nil {
		return auth, err
	}
	envelope, err := requestSub2API[sub2APITokenData](ctx, client.httpClient, sub2APIRequest{
		method: http.MethodPost,
		url:    client.rootURL + "/api/v1/auth/refresh",
		body:   payload,
	})
	if err != nil {
		return auth, fmt.Errorf("refresh sub2api token: %w", err)
	}
	return client.applyTokenData(auth, envelope.Data)
}

func (client *Sub2APIClient) applyTokenData(auth Sub2APIAuthState, token sub2APITokenData) (Sub2APIAuthState, error) {
	if strings.TrimSpace(token.AccessToken) == "" || strings.TrimSpace(token.RefreshToken) == "" || token.ExpiresIn <= 0 {
		return auth, errors.New("sub2api returned an incomplete token pair")
	}
	auth.AccessToken = token.AccessToken
	auth.RefreshToken = token.RefreshToken
	auth.AccessTokenExpiresAt = client.now().Add(time.Duration(token.ExpiresIn) * time.Second).Unix()
	return auth, nil
}
