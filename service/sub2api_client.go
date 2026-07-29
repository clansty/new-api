package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/sub2apiauth"
)

var ErrSub2API2FARequired = errors.New("sub2api account requires two-factor authentication")

type Sub2APIAuthState struct {
	Email                string
	Password             string
	AccessToken          string
	RefreshToken         string
	AccessTokenExpiresAt int64
}

type Sub2APIMetadata struct {
	RateMultiplier   float64
	GroupName        string
	GroupDescription string
}

type Sub2APIClient struct {
	rootURL    string
	httpClient *http.Client
	now        func() time.Time
}

type sub2APIRequest struct {
	method      string
	url         string
	accessToken string
	body        []byte
}

type sub2APIEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type sub2APIKey struct {
	Key   string        `json:"key"`
	Group *sub2APIGroup `json:"group"`
}

type sub2APIGroup struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

type sub2APIKeyPage struct {
	Items []sub2APIKey `json:"items"`
}

type sub2APIKeyBillingResponse struct {
	Object                  string   `json:"object"`
	SchemaVersion           int      `json:"schema_version"`
	BillingScope            string   `json:"billing_scope"`
	EffectiveRateMultiplier *float64 `json:"effective_rate_multiplier"`
}

func NewSub2APIClient(baseURL, proxyURL string) (*Sub2APIClient, error) {
	client, err := NewProxyHttpClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("create sub2api client: %w", err)
	}
	return &Sub2APIClient{
		rootURL:    sub2apiauth.NormalizeRootURL(baseURL),
		httpClient: client,
		now:        time.Now,
	}, nil
}

func (client *Sub2APIClient) QueryMetadata(ctx context.Context, apiKey string, auth Sub2APIAuthState) (Sub2APIMetadata, Sub2APIAuthState, error) {
	updatedAuth, err := client.ensureAccessToken(ctx, auth)
	if err != nil {
		return Sub2APIMetadata{}, auth, err
	}

	group, err := client.queryAPIKeyGroup(ctx, apiKey, updatedAuth.AccessToken)
	if err != nil {
		return Sub2APIMetadata{}, updatedAuth, err
	}
	rates, err := client.queryUserGroupRates(ctx, updatedAuth.AccessToken)
	if err != nil {
		return Sub2APIMetadata{}, updatedAuth, err
	}

	rateMultiplier := group.RateMultiplier
	if customRate, ok := rates[fmt.Sprintf("%d", group.ID)]; ok {
		rateMultiplier = customRate
	}
	return Sub2APIMetadata{
		RateMultiplier:   rateMultiplier,
		GroupName:        group.Name,
		GroupDescription: group.Description,
	}, updatedAuth, nil
}

func (client *Sub2APIClient) QueryDeclaredRateMultiplier(ctx context.Context, apiKey string) (float64, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.rootURL+"/v1/sub2api/billing", nil)
	if err != nil {
		return 0, fmt.Errorf("create sub2api billing request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return 0, fmt.Errorf("query sub2api declared billing: %w", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	closeErr := response.Body.Close()
	if readErr != nil {
		return 0, fmt.Errorf("read sub2api declared billing: %w", readErr)
	}
	if closeErr != nil {
		return 0, fmt.Errorf("close sub2api declared billing: %w", closeErr)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("query sub2api declared billing: %s", response.Status)
	}

	var result sub2APIKeyBillingResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("parse sub2api declared billing: %w", err)
	}
	if result.Object != "sub2api.key_billing" || result.SchemaVersion != 1 || result.BillingScope != "token" || result.EffectiveRateMultiplier == nil {
		return 0, errors.New("unexpected sub2api declared billing response")
	}
	rate := *result.EffectiveRateMultiplier
	if rate < 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 0, errors.New("invalid sub2api declared rate multiplier")
	}
	return rate, nil
}

func (client *Sub2APIClient) queryAPIKeyGroup(ctx context.Context, apiKey, accessToken string) (*sub2APIGroup, error) {
	query := url.Values{}
	query.Set("search", apiKey)
	query.Set("page", "1")
	query.Set("page_size", "100")
	endpoint := client.rootURL + "/api/v1/keys?" + query.Encode()
	envelope, err := requestSub2API[sub2APIKeyPage](ctx, client.httpClient, sub2APIRequest{
		method:      http.MethodGet,
		url:         endpoint,
		accessToken: accessToken,
	})
	if err != nil {
		return nil, fmt.Errorf("query sub2api API keys: %w", err)
	}
	for _, key := range envelope.Data.Items {
		if key.Key == apiKey && key.Group != nil {
			return key.Group, nil
		}
	}
	return nil, errors.New("sub2api API key or assigned group not found")
}

func (client *Sub2APIClient) queryUserGroupRates(ctx context.Context, accessToken string) (map[string]float64, error) {
	envelope, err := requestSub2API[map[string]float64](ctx, client.httpClient, sub2APIRequest{
		method:      http.MethodGet,
		url:         client.rootURL + "/api/v1/groups/rates",
		accessToken: accessToken,
	})
	if err != nil {
		return nil, fmt.Errorf("query sub2api group rates: %w", err)
	}
	return envelope.Data, nil
}

func requestSub2API[T any](ctx context.Context, client *http.Client, spec sub2APIRequest) (sub2APIEnvelope[T], error) {
	var result sub2APIEnvelope[T]
	request, err := http.NewRequestWithContext(ctx, spec.method, spec.url, bytes.NewReader(spec.body))
	if err != nil {
		return result, err
	}
	if len(spec.body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	if spec.accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+spec.accessToken)
	}
	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	closeErr := response.Body.Close()
	if readErr != nil {
		return result, readErr
	}
	if closeErr != nil {
		return result, closeErr
	}
	if err := common.Unmarshal(body, &result); err != nil {
		return result, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || result.Code != 0 {
		message := strings.TrimSpace(result.Message)
		if message == "" {
			message = response.Status
		}
		return result, errors.New(message)
	}
	return result, nil
}
