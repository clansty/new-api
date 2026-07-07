package operation_setting

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func withOptionMap(t *testing.T, values map[string]string) {
	t.Helper()

	common.OptionMapRWMutex.Lock()
	old := common.OptionMap
	common.OptionMap = values
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = old
		common.OptionMapRWMutex.Unlock()
	})
}

func TestGetAutoTestChannelRuntimeConfig_usesDefaultsWhenUnset(t *testing.T) {
	withOptionMap(t, map[string]string{})

	enabled, interval, minutes := GetAutoTestChannelRuntimeConfig()

	require.False(t, enabled)
	require.Equal(t, 10.0, minutes)
	require.Equal(t, 10*time.Minute, interval)
}

func TestGetAutoTestChannelRuntimeConfig_readsOptionMap(t *testing.T) {
	withOptionMap(t, map[string]string{
		"monitor_setting.auto_test_channel_enabled": "true",
		"monitor_setting.auto_test_channel_minutes": "2",
	})

	enabled, interval, minutes := GetAutoTestChannelRuntimeConfig()

	require.True(t, enabled)
	require.Equal(t, 2.0, minutes)
	require.Equal(t, 2*time.Minute, interval)
}

func TestGetAutoTestChannelRuntimeConfig_clampsInvalidInterval(t *testing.T) {
	withOptionMap(t, map[string]string{
		"monitor_setting.auto_test_channel_enabled": "true",
		"monitor_setting.auto_test_channel_minutes": "0",
	})

	enabled, interval, minutes := GetAutoTestChannelRuntimeConfig()

	require.True(t, enabled)
	require.Equal(t, 1.0, minutes)
	require.Equal(t, time.Minute, interval)
}

func TestGetAutoTestChannelRuntimeConfig_envOverridesOptionMap(t *testing.T) {
	t.Setenv("CHANNEL_TEST_FREQUENCY", "3")
	withOptionMap(t, map[string]string{
		"monitor_setting.auto_test_channel_enabled": "false",
		"monitor_setting.auto_test_channel_minutes": "10",
	})

	enabled, interval, minutes := GetAutoTestChannelRuntimeConfig()

	require.True(t, enabled)
	require.Equal(t, 3.0, minutes)
	require.Equal(t, 3*time.Minute, interval)
}
