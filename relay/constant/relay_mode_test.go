package constant

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPath2RelayMode_whenAlphaSearchAlias(t *testing.T) {
	testCases := []string{
		"/v1/alpha/search",
		"/alpha/search",
		"/backend-api/codex/alpha/search",
	}

	for _, path := range testCases {
		t.Run(path, func(t *testing.T) {
			// 三条客户端入口必须归一到同一 relay 能力，否则裸路径会落入前端 fallback。
			require.Equal(t, RelayModeAlphaSearch, Path2RelayMode(path))
		})
	}
}
