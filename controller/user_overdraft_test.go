package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func setupUserControllerTestDB(t *testing.T) {
	t.Helper()

	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}))
}

func seedAdminEditableUser(t *testing.T, allowOverdraft bool) *model.User {
	t.Helper()

	user := &model.User{
		Id:             2001,
		Username:       "editable_user",
		Password:       "password",
		DisplayName:    "Editable User",
		Group:          "default",
		Role:           common.RoleCommonUser,
		Status:         common.UserStatusEnabled,
		AllowOverdraft: allowOverdraft,
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func getUserAllowOverdraft(t *testing.T, userID int) bool {
	t.Helper()

	var user model.User
	require.NoError(t, model.DB.Select("allow_overdraft").Where("id = ?", userID).First(&user).Error)
	return user.AllowOverdraft
}

func TestUpdateUserPreservesAllowOverdraftWhenFieldMissing(t *testing.T) {
	setupUserControllerTestDB(t)
	user := seedAdminEditableUser(t, true)

	body := map[string]any{
		"id":           user.Id,
		"username":     user.Username,
		"password":     "",
		"display_name": "Renamed User",
		"group":        user.Group,
		"role":         user.Role,
		"remark":       "keep overdraft",
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", body, common.RoleRootUser)
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.True(t, getUserAllowOverdraft(t, user.Id))
}

func TestUpdateUserCanDisableAllowOverdraftExplicitly(t *testing.T) {
	setupUserControllerTestDB(t)
	user := seedAdminEditableUser(t, true)

	body := map[string]any{
		"id":              user.Id,
		"username":        user.Username,
		"password":        "",
		"display_name":    user.DisplayName,
		"group":           user.Group,
		"role":            user.Role,
		"remark":          "disable overdraft",
		"allow_overdraft": false,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", body, common.RoleRootUser)
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.False(t, getUserAllowOverdraft(t, user.Id))
}
