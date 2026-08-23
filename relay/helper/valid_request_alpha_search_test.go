package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetAndValidateRequest_whenAlphaSearch(t *testing.T) {
	newContext := func(body string) *gin.Context {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		return c
	}

	c := newContext(`{"future_field":{"keep":true}}`)
	_, err := GetAndValidateRequest(c, types.RelayFormatOpenAIAlphaSearch)
	require.Error(t, err)

	c = newContext(`{"model":"gpt-5.6-sol","future_field":{"keep":true}}`)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-5.6-sol")
	request, err := GetAndValidateRequest(c, types.RelayFormatOpenAIAlphaSearch)

	require.NoError(t, err)
	alphaRequest, ok := request.(*dto.AlphaSearchRequest)
	require.True(t, ok)
	require.Equal(t, "gpt-5.6-sol", alphaRequest.Model)
}

func TestGetAndValidateAlphaSearchRequest_acceptsArrayInput(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := `{"model":"gpt-5.6-luna","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"搜索新闻"}]}],"commands":{"search_query":[{"q":"今日科技新闻"}]}}`
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-5.6-luna")

	request, err := GetAndValidateAlphaSearchRequest(c)

	require.NoError(t, err)
	require.Equal(t, "gpt-5.6-luna", request.Model)
}
