package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPrepareAlphaSearchRequestBody_whenModelMapped(t *testing.T) {
	body := `{"id":"search-session","model":"gpt-5.6-sol","commands":{"search_query":[{"q":"news"}]},"future_field":{"keep":true},"max_output_tokens":0}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("model_mapping", `{"gpt-5.6-sol":"upstream-5.6"}`)
	request := &dto.AlphaSearchRequest{Model: "gpt-5.6-sol"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.6-sol",
		RelayFormat:     types.RelayFormatOpenAIAlphaSearch,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.6-sol",
		},
	}

	reader, err := prepareAlphaSearchRequestBody(c, info, request)
	require.NoError(t, err)
	got, err := io.ReadAll(reader)
	require.NoError(t, err)

	// alpha 协议仍在演进，只允许改写 model，未知字段和显式零值必须保留。
	require.Equal(t, "upstream-5.6", gjson.GetBytes(got, "model").String())
	require.True(t, gjson.GetBytes(got, "future_field.keep").Bool())
	require.True(t, gjson.GetBytes(got, "max_output_tokens").Exists())
	require.Equal(t, int64(0), gjson.GetBytes(got, "max_output_tokens").Int())
}
