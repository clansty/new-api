package controller

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type tokenModelPricing struct {
	ModelName         string             `json:"model_name"`
	BillingType       string             `json:"billing_type"`
	BillingExpr       string             `json:"billing_expr,omitempty"`
	Tiers             []tokenPricingTier `json:"tiers,omitempty"`
	InputPrice        *float64           `json:"input_price,omitempty"`
	OutputPrice       *float64           `json:"output_price,omitempty"`
	CacheReadPrice    *float64           `json:"cache_read_price,omitempty"`
	CacheWritePrice   *float64           `json:"cache_write_price,omitempty"`
	CacheWrite1hPrice *float64           `json:"cache_write_1h_price,omitempty"`
	FixedPrice        *float64           `json:"fixed_price,omitempty"`
	Multiplier        float64            `json:"multiplier"`
}

type tokenPricingTier struct {
	Label             string   `json:"label"`
	InputPrice        *float64 `json:"input_price,omitempty"`
	OutputPrice       *float64 `json:"output_price,omitempty"`
	CacheReadPrice    *float64 `json:"cache_read_price,omitempty"`
	CacheWritePrice   *float64 `json:"cache_write_price,omitempty"`
	CacheWrite1hPrice *float64 `json:"cache_write_1h_price,omitempty"`
}

var (
	tierTermPattern = regexp.MustCompile(`^\(?\s*(p|c|cr|cc|cc1h)\s*\)?\s*\*\s*([+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?)\s*$`)
)

type tierCall struct {
	label string
	body  string
}

func extractTierCalls(expr string) []tierCall {
	calls := make([]tierCall, 0)
	for len(expr) > 0 {
		index := strings.Index(expr, "tier(")
		if index < 0 {
			break
		}
		start := index + len("tier(")
		depth := 1
		end := -1
		inQuotes := false
		for i := start; i < len(expr); i++ {
			switch expr[i] {
			case '"':
				inQuotes = !inQuotes
			case '(':
				if !inQuotes {
					depth++
				}
			case ')':
				if !inQuotes {
					depth--
					if depth == 0 {
						end = i
					}
				}
			}
			if end >= 0 {
				break
			}
		}
		if end < 0 {
			break
		}
		args := expr[start:end]
		comma := strings.Index(args, ",")
		if comma <= 0 {
			expr = expr[end+1:]
			continue
		}
		label := strings.Trim(strings.TrimSpace(args[:comma]), "\"")
		calls = append(calls, tierCall{label: label, body: strings.TrimSpace(args[comma+1:])})
		expr = expr[end+1:]
	}
	return calls
}

func tokenPricingModels(token *model.Token, userGroup string) map[string]bool {
	if token.ModelLimitsEnabled {
		return token.GetModelLimitsMap()
	}

	group := token.Group
	if group == "" {
		group = userGroup
	}
	groups := []string{group}
	if group == "auto" {
		groups = service.GetUserAutoGroup(userGroup)
	}
	models := make(map[string]bool)
	for _, enabledGroup := range groups {
		for _, modelName := range model.GetGroupEnabledModels(enabledGroup) {
			models[modelName] = true
		}
	}
	return models
}

func tokenPricingMultiplier(token *model.Token, userGroup string, pricing model.Pricing) float64 {
	if token.CustomRatio > 0 {
		return token.CustomRatio
	}

	group := token.Group
	if group == "" {
		group = userGroup
	}
	if group == "auto" {
		for _, autoGroup := range service.GetUserAutoGroup(userGroup) {
			if !common.StringsContains(pricing.EnableGroup, autoGroup) {
				continue
			}
			return groupRatioForToken(userGroup, autoGroup, pricing.ModelName)
		}
		return 1
	}
	return groupRatioForToken(userGroup, group, pricing.ModelName)
}

func groupRatioForToken(userGroup, usingGroup, modelName string) float64 {
	if !ratio_setting.ShouldIgnoreGroupSpecialRatio(modelName) {
		if specialRatio, ok := ratio_setting.GetGroupGroupRatio(userGroup, usingGroup); ok {
			return specialRatio
		}
	}
	return ratio_setting.GetGroupRatio(usingGroup)
}

func multiplyPrice(value float64, multiplier float64) *float64 {
	result := value * multiplier
	return &result
}

func parseTieredPricing(expr string, multiplier float64) []tokenPricingTier {
	calls := extractTierCalls(expr)
	if len(calls) == 0 {
		return nil
	}
	tiers := make([]tokenPricingTier, 0, len(calls))
	for _, call := range calls {
		prices := make(map[string]float64)
		for _, term := range strings.Split(call.body, "+") {
			termMatch := tierTermPattern.FindStringSubmatch(strings.TrimSpace(term))
			if len(termMatch) != 3 {
				return nil
			}
			coefficient, err := strconv.ParseFloat(termMatch[2], 64)
			if err != nil {
				return nil
			}
			prices[termMatch[1]] = coefficient * multiplier
		}
		tier := tokenPricingTier{Label: call.label}
		if value, ok := prices["p"]; ok {
			tier.InputPrice = &value
		}
		if value, ok := prices["c"]; ok {
			tier.OutputPrice = &value
		}
		if value, ok := prices["cr"]; ok {
			tier.CacheReadPrice = &value
		}
		if value, ok := prices["cc"]; ok {
			tier.CacheWritePrice = &value
		}
		if value, ok := prices["cc1h"]; ok {
			tier.CacheWrite1hPrice = &value
		}
		tiers = append(tiers, tier)
	}
	return tiers
}

func buildTokenModelPricing(pricing model.Pricing, multiplier float64) tokenModelPricing {
	item := tokenModelPricing{ModelName: pricing.ModelName, Multiplier: multiplier}
	if pricing.QuotaType == 1 {
		item.BillingType = "fixed"
		item.FixedPrice = multiplyPrice(pricing.ModelPrice, multiplier)
		return item
	}
	if pricing.BillingMode == "tiered_expr" && strings.TrimSpace(pricing.BillingExpr) != "" {
		item.BillingType = "tiered"
		item.BillingExpr = pricing.BillingExpr
		item.Tiers = parseTieredPricing(pricing.BillingExpr, multiplier)
		return item
	}
	item.BillingType = "token"
	// 倍率 1 对应 $0.002/1K token，即 $2/1M token。
	inputPrice := pricing.ModelRatio * 2 * multiplier
	item.InputPrice = &inputPrice
	item.OutputPrice = multiplyPrice(inputPrice, pricing.CompletionRatio)
	if pricing.CacheRatio != nil {
		item.CacheReadPrice = multiplyPrice(inputPrice, *pricing.CacheRatio)
	}
	if pricing.CreateCacheRatio != nil {
		item.CacheWritePrice = multiplyPrice(inputPrice, *pricing.CreateCacheRatio)
	}
	return item
}

// GetTokenPricing 返回当前令牌实际可访问模型的最终价格。
func GetTokenPricing(c *gin.Context) {
	token, err := model.GetTokenById(c.GetInt("token_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "主密钥无效"})
		return
	}
	rootToken := token
	if token.ParentId > 0 {
		rootToken, err = model.GetTokenById(token.ParentId)
		if err != nil || rootToken.UserId != token.UserId || rootToken.ParentId != 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "主密钥无效"})
			return
		}
	}
	effectiveToken := model.EffectiveToken(token, rootToken)
	userGroup, err := model.GetUserGroup(effectiveToken.UserId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	accessibleModels := tokenPricingModels(effectiveToken, userGroup)
	pricingItems := model.GetPricing()
	items := make([]tokenModelPricing, 0, len(accessibleModels))
	for _, pricing := range pricingItems {
		if !accessibleModels[pricing.ModelName] || !helper.HasModelBillingConfig(pricing.ModelName) {
			continue
		}
		multiplier := tokenPricingMultiplier(effectiveToken, userGroup, pricing)
		item := buildTokenModelPricing(pricing, multiplier)
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"currency": "USD",
			"unit":     "1M tokens",
			"items":    items,
		},
	})
}
