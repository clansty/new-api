package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// 优先级重试策略
const (
	// RetryPriorityStrategySamePriority 同优先级优先：本优先级渠道全部失败后才降级到下一优先级（默认行为）
	RetryPriorityStrategySamePriority = "same"
	// RetryPriorityStrategyNextPriority 下一优先级：本优先级任一渠道失败后立即尝试下一优先级
	RetryPriorityStrategyNextPriority = "next"
)

// ModelRetryConfig 单个模型的重试配置
type ModelRetryConfig struct {
	// RetryTimes 该模型的失败重试次数；nil 表示沿用全局 RetryTimes
	RetryTimes *int `json:"retry_times,omitempty"`
	// PriorityStrategy 失败后的优先级选路策略，见 RetryPriorityStrategy* 常量；空视为 same
	PriorityStrategy string `json:"priority_strategy,omitempty"`
}

type ModelRetrySetting struct {
	// Models 模型名 -> 重试配置
	Models map[string]ModelRetryConfig `json:"models"`
}

var modelRetrySetting = ModelRetrySetting{
	Models: map[string]ModelRetryConfig{},
}

func init() {
	config.GlobalConfig.Register("model_retry_setting", &modelRetrySetting)
}

func GetModelRetrySetting() *ModelRetrySetting {
	return &modelRetrySetting
}

// GetModelRetryTimes 返回指定模型的重试次数；未单独配置时返回 fallback（全局值）
func GetModelRetryTimes(modelName string, fallback int) int {
	if cfg, ok := modelRetrySetting.Models[modelName]; ok && cfg.RetryTimes != nil {
		return *cfg.RetryTimes
	}
	return fallback
}

// UseNextPriorityOnFailure 指定模型失败后是否直接尝试下一优先级（而非在同优先级内重试）
func UseNextPriorityOnFailure(modelName string) bool {
	if cfg, ok := modelRetrySetting.Models[modelName]; ok {
		return cfg.PriorityStrategy == RetryPriorityStrategyNextPriority
	}
	return false
}
