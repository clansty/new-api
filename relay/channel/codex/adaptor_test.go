package codex

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdaptorGetRequestURL_whenAlphaSearch(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RequestURLPath: "/v1/alpha/search?feature=standalone",
		RelayFormat:    types.RelayFormatOpenAIAlphaSearch,
		RelayMode:      relayconstant.RelayModeAlphaSearch,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCodex,
			ChannelBaseUrl: "https://chatgpt.com",
		},
	}

	url, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	require.Equal(t, "https://chatgpt.com/backend-api/codex/alpha/search?feature=standalone", url)
}

func TestAdaptorSetupRequestHeader_whenAlphaSearch(t *testing.T) {
	key, err := common.Marshal(OAuthKey{
		AccessToken: "oauth-token",
		AccountID:   "account-id",
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", nil)
	c.Request.Header.Set("Version", "0.144.1")
	header := http.Header{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAlphaSearch,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: string(key),
		},
	}

	err = (&Adaptor{}).SetupRequestHeader(c, &header, info)

	require.NoError(t, err)
	require.Equal(t, "Bearer oauth-token", header.Get("Authorization"))
	require.Equal(t, "account-id", header.Get("chatgpt-account-id"))
	require.Equal(t, "0.144.1", header.Get("Version"))
	require.Empty(t, header.Get("OpenAI-Beta"))
	require.Equal(t, "application/json", header.Get("Accept"))
}

func TestAdaptorSetupRequestHeader_whenAlphaSearchBetaProvided(t *testing.T) {
	key, err := common.Marshal(OAuthKey{
		AccessToken: "oauth-token",
		AccountID:   "account-id",
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", nil)
	c.Request.Header.Set("OpenAI-Beta", "search=v1")
	header := http.Header{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAlphaSearch,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: string(key),
		},
	}

	err = (&Adaptor{}).SetupRequestHeader(c, &header, info)

	require.NoError(t, err)
	require.Equal(t, "search=v1", header.Get("OpenAI-Beta"))
}
