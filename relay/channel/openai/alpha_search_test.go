package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

func TestAdaptorGetRequestURL_whenAlphaSearchAlias(t *testing.T) {
	for _, path := range []string{
		"/v1/alpha/search?feature=standalone",
		"/alpha/search?feature=standalone",
		"/backend-api/codex/alpha/search?feature=standalone",
	} {
		t.Run(path, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				RequestURLPath: path,
				RelayFormat:    types.RelayFormatOpenAIAlphaSearch,
				RelayMode:      relayconstant.RelayModeAlphaSearch,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:    constant.ChannelTypeOpenAI,
					ChannelBaseUrl: "https://upstream.example",
				},
			}

			url, err := (&Adaptor{}).GetRequestURL(info)

			// API Key 上游只接受公开 API 路径，不能把 Codex OAuth 别名原样传过去。
			require.NoError(t, err)
			require.Equal(t, "https://upstream.example/v1/alpha/search?feature=standalone", url)
		})
	}
}

func TestAdaptorGetRequestURL_whenAzureAlphaSearch(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RequestURLPath: "/v1/alpha/search",
		RelayFormat:    types.RelayFormatOpenAIAlphaSearch,
		RelayMode:      relayconstant.RelayModeAlphaSearch,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeAzure,
			ChannelBaseUrl: "https://example.openai.azure.com",
		},
	}

	url, err := (&Adaptor{}).GetRequestURL(info)

	// Azure 没有该 alpha 协议，必须在发出错误部署路径前拒绝。
	require.ErrorContains(t, err, "alpha search")
	require.Empty(t, url)
}
