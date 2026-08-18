package router

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetApiRouter_registersGroupRootRoutesWithoutTrailingSlash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetApiRouter(router)

	routes := make(map[string]bool)
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	for _, route := range []string{
		http.MethodGet + " /api/user",
		http.MethodPost + " /api/user",
		http.MethodPut + " /api/user",
		http.MethodGet + " /api/user/:id/tokens",
		http.MethodPost + " /api/user/:id/tokens",
		http.MethodPut + " /api/user/:id/tokens",
		http.MethodGet + " /api/option",
		http.MethodPut + " /api/option",
		http.MethodGet + " /api/custom-oauth-provider",
		http.MethodPost + " /api/custom-oauth-provider",
		http.MethodGet + " /api/channel",
		http.MethodPost + " /api/channel",
		http.MethodPut + " /api/channel",
		http.MethodGet + " /api/token",
		http.MethodPost + " /api/token",
		http.MethodPut + " /api/token",
		http.MethodGet + " /api/usage/token",
		http.MethodGet + " /api/redemption",
		http.MethodPost + " /api/redemption",
		http.MethodPut + " /api/redemption",
		http.MethodGet + " /api/log",
		http.MethodDelete + " /api/log",
		http.MethodGet + " /api/data",
		http.MethodGet + " /api/group",
		http.MethodGet + " /api/prefill_group",
		http.MethodPost + " /api/prefill_group",
		http.MethodPut + " /api/prefill_group",
		http.MethodGet + " /api/mj",
		http.MethodGet + " /api/task",
		http.MethodGet + " /api/vendors",
		http.MethodPost + " /api/vendors",
		http.MethodPut + " /api/vendors",
		http.MethodGet + " /api/models/manage",
		http.MethodPost + " /api/models",
		http.MethodPut + " /api/models",
		http.MethodGet + " /api/deployments",
		http.MethodPost + " /api/deployments",
	} {
		require.True(t, routes[route], "%s should be registered without a trailing slash", route)
		require.True(t, routes[route+"/"], "%s/ should remain registered for compatibility", route)
	}
}

func TestRouters_registerEveryTrailingSlashRouteWithoutTrailingSlash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetApiRouter(router)
	SetVideoRouter(router)

	routes := make(map[string]bool)
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	for _, route := range router.Routes() {
		if route.Path == "/api/models/" {
			require.True(t, routes[http.MethodGet+" /api/models/manage"])
			continue
		}
		if route.Path == "/" || !strings.HasSuffix(route.Path, "/") {
			continue
		}

		pathWithoutTrailingSlash := strings.TrimSuffix(route.Path, "/")
		require.True(
			t,
			routes[route.Method+" "+pathWithoutTrailingSlash],
			"%s %s should also be registered without a trailing slash",
			route.Method,
			route.Path,
		)
	}
}
