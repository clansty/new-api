package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestInitLogDB_usesPostgreSQLDialect_whenSharingPostgreSQLMainDB(t *testing.T) {
	// Given: 主库为 PostgreSQL，日志库未配置独立连接。
	originalUsingSQLite := common.UsingSQLite
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalUsingMySQL := common.UsingMySQL
	originalLogSQLType := common.LogSqlType
	originalLogDB := LOG_DB
	t.Cleanup(func() {
		common.UsingSQLite = originalUsingSQLite
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.UsingMySQL = originalUsingMySQL
		common.LogSqlType = originalLogSQLType
		LOG_DB = originalLogDB
		initCol()
	})

	t.Setenv("LOG_SQL_DSN", "")
	common.UsingSQLite = false
	common.UsingPostgreSQL = true
	common.UsingMySQL = false
	common.LogSqlType = common.DatabaseTypeSQLite
	initCol()

	// When: 日志库复用主库连接。
	err := InitLogDB()

	// Then: 日志查询沿用 PostgreSQL 方言。
	require.NoError(t, err)
	require.Same(t, DB, LOG_DB)
	require.Equal(t, common.DatabaseTypePostgreSQL, common.LogSqlType)
	require.Equal(t, `"group"`, logGroupCol)
}
