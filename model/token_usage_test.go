package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMigrateSubTokenUsedQuotaUsesSeparateLogDBOnce(t *testing.T) {
	truncateTables(t)
	originalLogDB := LOG_DB
	logDB, err := gorm.Open(sqlite.Open("file:token-usage-log?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, logDB.AutoMigrate(&Log{}))
	LOG_DB = logDB
	t.Cleanup(func() {
		LOG_DB = originalLogDB
		sqlDB, dbErr := logDB.DB()
		if dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	parent := Token{UserId: 1, Key: "usage-parent", Status: common.TokenStatusEnabled}
	require.NoError(t, DB.Create(&parent).Error)
	child := Token{UserId: 1, ParentId: parent.Id, Key: "usage-child", Status: common.TokenStatusEnabled}
	require.NoError(t, DB.Create(&child).Error)
	require.NoError(t, LOG_DB.Create(&[]Log{
		{TokenId: child.Id, Type: LogTypeConsume, Quota: 50, Other: common.MapToJsonStr(map[string]any{"token_quota": 70})},
		{TokenId: child.Id, Type: LogTypeConsume, Quota: 30},
		{TokenId: child.Id, Type: LogTypeRefund, Quota: 10, Other: common.MapToJsonStr(map[string]any{"token_quota": 20})},
	}).Error)

	require.NoError(t, MigrateSubTokenUsedQuota())
	require.NoError(t, DB.First(&child, child.Id).Error)
	require.Equal(t, 80, child.UsedQuota)
	require.True(t, child.UsedQuotaInitialized)

	require.NoError(t, LOG_DB.Create(&Log{TokenId: child.Id, Type: LogTypeConsume, Quota: 500}).Error)
	require.NoError(t, MigrateSubTokenUsedQuota())
	require.NoError(t, DB.First(&child, child.Id).Error)
	require.Equal(t, 80, child.UsedQuota)
}
