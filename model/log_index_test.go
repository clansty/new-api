package model

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

func TestLogIndexesSupportLatestTokenQuery(t *testing.T) {
	// Given: 一个日志模型的 GORM schema。
	parsed, err := schema.Parse(&Log{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)

	// When: 读取模型声明的索引定义。
	indexes := parsed.ParseIndexes()

	// Then: token 过滤和最新 id 排序各自拥有复合索引。
	byName := make(map[string][]string, len(indexes))
	for name, index := range indexes {
		columns := make([]string, 0, len(index.Fields))
		for _, field := range index.Fields {
			columns = append(columns, field.Name)
		}
		byName[name] = columns
	}
	require.Equal(t, []string{"TokenId", "Id"}, byName["idx_logs_token_id_id_desc"])
	require.Equal(t, []string{"TokenRootId", "Id"}, byName["idx_logs_token_root_id_id_desc"])
}
