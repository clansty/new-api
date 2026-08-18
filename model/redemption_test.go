package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestRedeemAppliesQuotaAndGroup(t *testing.T) {
	// Given
	require.NoError(t, DB.AutoMigrate(&Redemption{}))
	user := &User{Username: "redeem-group-user", Password: "password", Group: "default"}
	require.NoError(t, DB.Create(user).Error)
	redemption := &Redemption{Key: "redeem-group-key", Status: common.RedemptionCodeStatusEnabled, Quota: 123, Group: "vip"}
	require.NoError(t, DB.Create(redemption).Error)
	t.Cleanup(func() {
		DB.Delete(&Redemption{}, redemption.Id)
		DB.Delete(&User{}, user.Id)
	})

	// When
	quota, err := Redeem(redemption.Key, user.Id)

	// Then
	require.NoError(t, err)
	require.Equal(t, 123, quota)
	var updated User
	require.NoError(t, DB.First(&updated, user.Id).Error)
	require.Equal(t, 123, updated.Quota)
	require.Equal(t, "vip", updated.Group)
}
