package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetChannel_initialSelectionPreservesResponseTimeout(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyChannelType, 1)
	common.SetContextKey(c, constant.ContextKeyChannelName, "slow")
	common.SetContextKey(c, constant.ContextKeyChannelResponseTimeout, 3)

	channel, err := getChannel(c, &relaycommon.RelayInfo{}, &service.RetryParam{})

	require.Nil(t, err)
	require.Equal(t, 3, channel.GetResponseTimeout())
}
