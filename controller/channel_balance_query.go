package controller

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

type sub2APIQuota struct {
	Remaining *float64 `json:"remaining,omitempty"`
}

type sub2APIUsageResponse struct {
	Remaining *float64      `json:"remaining,omitempty"`
	Balance   *float64      `json:"balance,omitempty"`
	Quota     *sub2APIQuota `json:"quota,omitempty"`
	Unit      string        `json:"unit,omitempty"`
}

type hyl2APIQuotaBucket struct {
	Remaining *float64 `json:"remaining,omitempty"`
}

type hyl2APIQuotaResponse struct {
	Success *bool               `json:"success,omitempty"`
	Detail  string              `json:"detail,omitempty"`
	USD     *hyl2APIQuotaBucket `json:"usd,omitempty"`
}

func ensureChannelBalanceBaseURL(channel *model.Channel) string {
	baseURL := channel.GetBaseURL()
	if baseURL == "" && channel.Type >= 0 && channel.Type < len(constant.ChannelBaseURLs) {
		baseURL = constant.ChannelBaseURLs[channel.Type]
		channel.BaseURL = &baseURL
	}
	return baseURL
}

func updateChannelBalanceByQueryMode(channel *model.Channel) (float64, bool, error) {
	switch mode := channel.GetSetting().BalanceQueryMode; mode {
	case dto.BalanceQueryModeDefault:
		return 0, false, nil
	case dto.BalanceQueryModeOpenAICompatible:
		balance, err := queryOpenAICompatibleBalance(channel)
		if err == nil {
			channel.UpdateBalance(balance)
		}
		return balance, true, err
	case dto.BalanceQueryModeSub2API:
		balance, err := querySub2APIBalance(channel)
		if err == nil {
			channel.UpdateBalance(balance)
		}
		return balance, true, err
	case dto.BalanceQueryModeHYL2API:
		balance, err := queryHYL2APIBalance(channel)
		if err == nil {
			channel.UpdateBalance(balance)
		}
		return balance, true, err
	case dto.BalanceQueryModeDisabled:
		return channel.Balance, true, nil
	default:
		return 0, true, fmt.Errorf("未知余额查询模式: %s", mode)
	}
}

func shouldSkipChannelBalanceUpdate(channel *model.Channel) bool {
	return channel.GetSetting().BalanceQueryMode == dto.BalanceQueryModeDisabled
}

func joinUpstreamPath(baseURL string, path string) string {
	trimmedBase := strings.TrimRight(baseURL, "/")
	trimmedPath := strings.TrimLeft(path, "/")
	if trimmedBase == "" {
		return "/" + trimmedPath
	}
	if strings.HasSuffix(trimmedBase, "/v1") && strings.HasPrefix(trimmedPath, "v1/") {
		trimmedPath = strings.TrimPrefix(trimmedPath, "v1/")
	}
	return trimmedBase + "/" + trimmedPath
}

func joinUpstreamRootPath(baseURL string, path string) string {
	trimmedBase := strings.TrimRight(baseURL, "/")
	trimmedBase = strings.TrimSuffix(trimmedBase, "/v1")
	trimmedPath := strings.TrimLeft(path, "/")
	if trimmedBase == "" {
		return "/" + trimmedPath
	}
	return trimmedBase + "/" + trimmedPath
}

func queryOpenAICompatibleBalance(channel *model.Channel) (float64, error) {
	return queryOpenAICompatibleBalanceAt(channel, time.Now())
}

func queryOpenAICompatibleBalanceAt(channel *model.Channel, now time.Time) (float64, error) {
	baseURL := channel.GetBaseURL()
	subscriptionURL := joinUpstreamPath(baseURL, "/v1/dashboard/billing/subscription")
	body, err := GetResponseBody("GET", subscriptionURL, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	subscription := OpenAISubscriptionResponse{}
	if err := common.Unmarshal(body, &subscription); err != nil {
		return 0, err
	}

	startDate := fmt.Sprintf("%s-01", now.Format("2006-01"))
	endDate := now.Format("2006-01-02")
	if !subscription.HasPaymentMethod {
		startDate = now.AddDate(0, 0, -100).Format("2006-01-02")
	}
	usageURL := fmt.Sprintf(
		"%s?start_date=%s&end_date=%s",
		joinUpstreamPath(baseURL, "/v1/dashboard/billing/usage"),
		startDate,
		endDate,
	)
	body, err = GetResponseBody("GET", usageURL, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	usage := OpenAIUsageResponse{}
	if err := common.Unmarshal(body, &usage); err != nil {
		return 0, err
	}
	return subscription.HardLimitUSD - usage.TotalUsage/100, nil
}

func querySub2APIBalance(channel *model.Channel) (float64, error) {
	url := joinUpstreamPath(channel.GetBaseURL(), "/v1/usage")
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := sub2APIUsageResponse{}
	if err := common.Unmarshal(body, &response); err != nil {
		return 0, err
	}
	return response.remainingBalance()
}

func (response sub2APIUsageResponse) remainingBalance() (float64, error) {
	if response.Unit != "" && !strings.EqualFold(response.Unit, "USD") {
		return 0, fmt.Errorf("unsupported sub2api balance unit: %s", response.Unit)
	}
	if response.Remaining != nil {
		return *response.Remaining, nil
	}
	if response.Quota != nil && response.Quota.Remaining != nil {
		return *response.Quota.Remaining, nil
	}
	if response.Balance != nil {
		return *response.Balance, nil
	}
	return 0, errors.New("sub2api usage response missing remaining balance")
}

func queryHYL2APIBalance(channel *model.Channel) (float64, error) {
	url := joinUpstreamRootPath(channel.GetBaseURL(), "/user/api/quota")
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := hyl2APIQuotaResponse{}
	if err := common.Unmarshal(body, &response); err != nil {
		return 0, err
	}
	return response.remainingBalance()
}

func (response hyl2APIQuotaResponse) remainingBalance() (float64, error) {
	if response.Success != nil && !*response.Success {
		if response.Detail != "" {
			return 0, fmt.Errorf("hyl2api quota query failed: %s", response.Detail)
		}
		return 0, errors.New("hyl2api quota query failed")
	}
	if response.USD != nil && response.USD.Remaining != nil {
		return *response.USD.Remaining, nil
	}
	return 0, errors.New("hyl2api quota response missing usd remaining balance")
}
