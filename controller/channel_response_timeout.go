package controller

import (
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func relayWithChannelResponseTimeout(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	channel *model.Channel,
	handler func() *types.NewAPIError,
) (relayErr *types.NewAPIError) {
	responseTimeout := channel.GetResponseTimeout()
	if responseTimeout <= 0 || !info.ShouldUseChannelResponseTimeout() {
		return handler()
	}

	originalRequest := c.Request
	duration := time.Duration(responseTimeout) * time.Second
	timeout := relaycommon.NewChannelResponseTimeout(originalRequest.Context(), duration)
	c.Request = originalRequest.WithContext(timeout.Context())
	defer func() {
		timedOut := timeout.TimedOut()
		timeout.Stop()
		c.Request = originalRequest
		if timedOut {
			relayErr = types.NewErrorWithStatusCode(
				&relaycommon.ChannelResponseTimeoutError{Timeout: duration},
				types.ErrorCodeChannelResponseTimeExceeded,
				http.StatusGatewayTimeout,
			)
		}
	}()

	return handler()
}
