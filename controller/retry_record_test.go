package controller

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldRetryAndRecord_whenChannelTimeoutHasRetryRemaining(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(nil)
	retryParam := &service.RetryParam{}
	timeoutErr := types.NewError(errors.New("timeout"), types.ErrorCodeChannelResponseTimeExceeded)

	shouldRetry := shouldRetryAndRecord(c, timeoutErr, 1, retryParam, 7)

	require.True(t, shouldRetry)
	require.True(t, retryParam.RequireDifferentChannel)
	require.Contains(t, retryParam.FailedChannelIDs, 7)
}

func TestShouldRetryAndRecord_whenChannelTimeoutIsLastAttempt(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(nil)
	retryParam := &service.RetryParam{}
	timeoutErr := types.NewError(errors.New("timeout"), types.ErrorCodeChannelResponseTimeExceeded)

	shouldRetry := shouldRetryAndRecord(c, timeoutErr, 0, retryParam, 7)

	require.False(t, shouldRetry)
	require.False(t, retryParam.RequireDifferentChannel)
	require.Empty(t, retryParam.FailedChannelIDs)
}
