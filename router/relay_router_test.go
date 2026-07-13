package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetRelayRouter_whenAlphaSearchAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetRelayRouter(router)

	routes := make(map[string]bool)
	for _, route := range router.Routes() {
		if route.Method == http.MethodPost {
			routes[route.Path] = true
		}
	}

	for _, path := range []string{
		"/v1/alpha/search",
		"/alpha/search",
		"/backend-api/codex/alpha/search",
	} {
		// Codex 会随 base_url 形态选择不同入口，三条路径缺一都会产生兼容性回归。
		require.True(t, routes[path], "POST %s should be registered", path)
	}
}
