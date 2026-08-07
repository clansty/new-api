package common

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestRelayInfoCloneForAttempt_keepsRequestMutationsIsolated(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{Model: "gpt-original"}
	info := &RelayInfo{OriginModelName: request.Model, Request: request}

	clone, err := info.CloneForAttempt()
	require.NoError(t, err)
	clone.Request.SetModelName("gpt-member")

	require.Equal(t, "gpt-original", request.Model)
	require.Equal(t, "gpt-member", clone.Request.(*dto.GeneralOpenAIRequest).Model)
}
