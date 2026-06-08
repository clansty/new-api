package passthrough

import (
	"io"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/types"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdaptorGetRequestURL_whenGeminiModelIsMapped(t *testing.T) {
	// Given: a Gemini request path containing the client-facing model name.
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RequestURLPath:  "/v1beta/models/client-model:streamGenerateContent?alt=sse",
		OriginModelName: "client-model",
		RelayFormat:     types.RelayFormatGemini,
		RelayMode:       relayconstant.RelayModeGemini,
		IsStream:        true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://upstream.example",
			UpstreamModelName: "gemini-2.5-pro",
		},
	}

	// When: the upstream request URL is built.
	url, err := adaptor.GetRequestURL(info)

	// Then: the path stays Gemini-native and uses the mapped upstream model.
	require.NoError(t, err)
	require.Equal(t, "https://upstream.example/v1beta/models/gemini-2.5-pro:streamGenerateContent?alt=sse", url)
}

func TestAdaptorGetRequestURL_whenClientCredentialQueryIsPresent(t *testing.T) {
	// Given: a Gemini request authenticated with a client key in the query string.
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RequestURLPath:  "/v1beta/models/gemini-2.5-pro:generateContent?key=client-token&alt=sse",
		OriginModelName: "gemini-2.5-pro",
		RelayFormat:     types.RelayFormatGemini,
		RelayMode:       relayconstant.RelayModeGemini,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://upstream.example",
			UpstreamModelName: "gemini-2.5-pro",
		},
	}

	// When: the upstream request URL is built.
	url, err := adaptor.GetRequestURL(info)

	// Then: client credentials are stripped while protocol query parameters remain.
	require.NoError(t, err)
	require.Equal(t, "https://upstream.example/v1beta/models/gemini-2.5-pro:generateContent?alt=sse", url)
}

func TestAdaptorGetRequestURL_whenRelayFormatIsUnsupported(t *testing.T) {
	// Given: a pass-through channel receives a request format outside its protocol set.
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RequestURLPath: "/v1/images/generations",
		RelayFormat:    types.RelayFormatOpenAIImage,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://upstream.example",
		},
	}

	// When: the upstream URL is requested.
	url, err := adaptor.GetRequestURL(info)

	// Then: no upstream URL is returned, so the payload is not forwarded first.
	require.ErrorContains(t, err, "unsupported pass-through relay format")
	require.Empty(t, url)
}

func TestAdaptorSetupRequestHeader_whenRelayFormats(t *testing.T) {
	testCases := []struct {
		name       string
		format     types.RelayFormat
		wantHeader string
		wantValue  string
	}{
		{name: "openai", format: types.RelayFormatOpenAI, wantHeader: "Authorization", wantValue: "Bearer test-key"},
		{name: "responses", format: types.RelayFormatOpenAIResponses, wantHeader: "Authorization", wantValue: "Bearer test-key"},
		{name: "claude", format: types.RelayFormatClaude, wantHeader: "x-api-key", wantValue: "test-key"},
		{name: "gemini", format: types.RelayFormatGemini, wantHeader: "x-goog-api-key", wantValue: "test-key"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given: a pass-through adaptor and an inbound request.
			adaptor := &Adaptor{}
			c := &gin.Context{Request: httptestRequestWithHeaders()}
			info := &relaycommon.RelayInfo{
				RelayFormat: testCase.format,
				ChannelMeta: &relaycommon.ChannelMeta{
					ApiKey: "test-key",
				},
			}
			header := http.Header{}

			// When: upstream headers are prepared.
			err := adaptor.SetupRequestHeader(c, &header, info)

			// Then: the protocol-native credential header is set.
			require.NoError(t, err)
			require.Equal(t, testCase.wantValue, header.Get(testCase.wantHeader))
		})
	}
}

func TestAdaptorDoRequest_whenRelayFormatIsUnsupported(t *testing.T) {
	// Given: a pass-through adaptor handling a non-supported relay format.
	adaptor := &Adaptor{}
	c := &gin.Context{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatRerank,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://upstream.example",
		},
	}

	// When: the adaptor is asked to send the upstream request.
	resp, err := adaptor.DoRequest(c, info, io.Reader(nil))

	// Then: the request is rejected before any upstream I/O can happen.
	require.Error(t, err)
	require.Nil(t, resp)
	require.Contains(t, err.Error(), "unsupported pass-through relay format")
}

func httptestRequestWithHeaders() *http.Request {
	req, err := http.NewRequest(http.MethodPost, "/v1/messages", nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	return req
}
