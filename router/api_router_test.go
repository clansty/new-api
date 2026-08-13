package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetApiRouter_registersLogQueryWithoutTrailingSlash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetApiRouter(router)

	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/log" {
			return
		}
	}

	require.Fail(t, "GET /api/log should be registered so proxies that strip trailing slashes do not cause a redirect loop")
}
