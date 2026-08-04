package service

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

func TestShouldDisableChannel_doesNotDisable_whenConfiguredResponseTimeoutExpires(t *testing.T) {
	original := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = original })

	timeoutErr := types.NewErrorWithStatusCode(
		&relaycommon.ChannelResponseTimeoutError{Timeout: time.Second},
		types.ErrorCodeChannelResponseTimeExceeded,
		http.StatusGatewayTimeout,
	)

	require.False(t, ShouldDisableChannel(timeoutErr))
}

func TestShouldDisableChannel_disables_whenAutomaticChannelTestExceedsThreshold(t *testing.T) {
	original := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = original })

	timeoutErr := types.NewOpenAIError(
		errors.New("响应时间超过自动禁用阈值"),
		types.ErrorCodeChannelResponseTimeExceeded,
		http.StatusRequestTimeout,
	)

	require.True(t, ShouldDisableChannel(timeoutErr))
}
