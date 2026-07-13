package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	request, err := GetAndValidateRequest(c, types.RelayFormatOpenAIAlphaSearch)

	require.NoError(t, err)
	alphaRequest, ok := request.(*dto.AlphaSearchRequest)
	require.True(t, ok)
	require.Equal(t, "gpt-5.6-sol", alphaRequest.Model)
}
