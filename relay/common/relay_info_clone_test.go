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

func TestRelayInfoCloneForAttempt_isolatesClaudeStreamState(t *testing.T) {
	info := &RelayInfo{
		ClaudeConvertInfo: &ClaudeConvertInfo{
			LastMessagesType: LastMessageTypeThinking,
			ReasoningContent: "首个 delta",
		},
	}

	clone, err := info.CloneForAttempt()
	require.NoError(t, err)
	clone.ClaudeConvertInfo.Done = true
	clone.ClaudeConvertInfo.ReasoningContent = "成员 A 的状态"

	require.False(t, info.ClaudeConvertInfo.Done)
	require.Equal(t, "首个 delta", info.ClaudeConvertInfo.ReasoningContent)
}
