package operation_setting

import (
	"math"
	"os"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const (
	defaultAutoTestChannelMinutes = 10
	minAutoTestChannelMinutes     = 1
)

type MonitorSetting struct {
	AutoTestChannelEnabled bool    `json:"auto_test_channel_enabled"`
	AutoTestChannelMinutes float64 `json:"auto_test_channel_minutes"`
}

// 默认配置
var monitorSetting = MonitorSetting{
	AutoTestChannelEnabled: false,
	AutoTestChannelMinutes: defaultAutoTestChannelMinutes,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("monitor_setting", &monitorSetting)
}

func GetMonitorSetting() *MonitorSetting {
	return &monitorSetting
}

func GetAutoTestChannelRuntimeConfig() (bool, time.Duration, float64) {
	enabled := false
	minutes := float64(defaultAutoTestChannelMinutes)

	common.OptionMapRWMutex.RLock()
	if value, ok := common.OptionMap["monitor_setting.auto_test_channel_enabled"]; ok {
		if parsed, err := strconv.ParseBool(value); err == nil {
			enabled = parsed
		}
	}
	if value, ok := common.OptionMap["monitor_setting.auto_test_channel_minutes"]; ok {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			minutes = parsed
		}
	}
	common.OptionMapRWMutex.RUnlock()

	if os.Getenv("CHANNEL_TEST_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_TEST_FREQUENCY"))
		if err == nil && frequency > 0 {
			enabled = true
			minutes = float64(frequency)
		}
	}

	intervalMinutes := normalizeAutoTestChannelMinutes(minutes)
	return enabled, time.Duration(intervalMinutes) * time.Minute, float64(intervalMinutes)
}

func normalizeAutoTestChannelMinutes(minutes float64) int {
	if math.IsNaN(minutes) || math.IsInf(minutes, 0) {
		return defaultAutoTestChannelMinutes
	}
	rounded := int(math.Round(minutes))
	if rounded < minAutoTestChannelMinutes {
		return minAutoTestChannelMinutes
	}
	return rounded
}
