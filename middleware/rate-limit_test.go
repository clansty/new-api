package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGlobalAPIRateLimitSkipsCodexManagementAPIs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalEnabled := common.GlobalApiRateLimitEnable
	originalNum := common.GlobalApiRateLimitNum
	originalDuration := common.GlobalApiRateLimitDuration
	originalRedisEnabled := common.RedisEnabled
	common.GlobalApiRateLimitEnable = true
	common.GlobalApiRateLimitNum = 1
	common.GlobalApiRateLimitDuration = 60
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.GlobalApiRateLimitEnable = originalEnabled
		common.GlobalApiRateLimitNum = originalNum
		common.GlobalApiRateLimitDuration = originalDuration
		common.RedisEnabled = originalRedisEnabled
	})

	tests := []struct {
		name string
		path string
	}{
		{name: "usage", path: "/api/usage/token"},
		{name: "pricing", path: "/api/pricing/token"},
		{name: "subkeys", path: "/api/token/subkeys"},
		{name: "subkey descendant", path: "/api/token/subkeys/12/key"},
		{name: "logs", path: "/api/log/token"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(GlobalAPIRateLimit())
			router.Any("/*path", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			for range 2 {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodGet, test.path, nil)
				request.RemoteAddr = "198.51.100.20:1234"
				router.ServeHTTP(recorder, request)
				require.Equal(t, http.StatusNoContent, recorder.Code)
			}
		})
	}
}

func TestGlobalAPIRateLimitStillLimitsUnrelatedAPIs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalEnabled := common.GlobalApiRateLimitEnable
	originalNum := common.GlobalApiRateLimitNum
	originalDuration := common.GlobalApiRateLimitDuration
	originalRedisEnabled := common.RedisEnabled
	common.GlobalApiRateLimitEnable = true
	common.GlobalApiRateLimitNum = 1
	common.GlobalApiRateLimitDuration = 60
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.GlobalApiRateLimitEnable = originalEnabled
		common.GlobalApiRateLimitNum = originalNum
		common.GlobalApiRateLimitDuration = originalDuration
		common.RedisEnabled = originalRedisEnabled
	})

	router := gin.New()
	router.Use(GlobalAPIRateLimit())
	router.GET("/api/status", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for index, expectedStatus := range []int{http.StatusNoContent, http.StatusTooManyRequests} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		request.RemoteAddr = "198.51.100.21:1234"
		router.ServeHTTP(recorder, request)
		require.Equalf(t, expectedStatus, recorder.Code, "request %d", index+1)
	}
}
