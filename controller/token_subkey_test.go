package controller

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSubTokenCreateInheritsParentPolicy(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	parent := seedToken(t, db, 1, "parent", "parent-key")
	parent.RemainQuota = 321
	parent.UnlimitedQuota = false
	parent.ModelLimitsEnabled = true
	parent.ModelLimits = "gpt-4o,claude-3"
	parent.AllowIps = func() *string { value := "10.0.0.0/8"; return &value }()
	parent.Group = "premium"
	parent.CrossGroupRetry = true
	parent.CustomRatio = 1.5
	require.NoError(t, db.Save(parent).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/1/subkeys", map[string]string{"name": "ci"}, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(parent.Id)}}
	CreateSubToken(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success)
	var created struct {
		ID  int    `json:"id"`
		Key string `json:"key"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &created))
	require.NotEmpty(t, created.Key)

	child, root, err := model.ResolveUserToken(created.Key[3:])
	require.NoError(t, err)
	require.Equal(t, parent.Id, child.ParentId)
	require.Equal(t, parent.Id, root.Id)
	effective := model.EffectiveToken(child, root)
	require.Equal(t, parent.RemainQuota, effective.RemainQuota)
	require.Equal(t, parent.ModelLimits, effective.ModelLimits)
	require.Equal(t, parent.CustomRatio, effective.CustomRatio)
	require.Equal(t, parent.Group, effective.Group)
}

func TestParentTokenLogsIncludeSubTokenLogs(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	parent := seedToken(t, db, 1, "parent", "parent-key")
	child, err := model.CreateSubToken(parent.Id, parent.UserId, "child")
	require.NoError(t, err)
	require.NoError(t, db.Create(&[]model.Log{
		{UserId: 1, TokenId: parent.Id, TokenRootId: parent.Id, Type: model.LogTypeConsume},
		{UserId: 1, TokenId: child.Id, TokenRootId: parent.Id, Type: model.LogTypeConsume},
	}).Error)

	parentLogs, err := model.GetLogByTokenId(parent.Id, parent.Id)
	require.NoError(t, err)
	require.Len(t, parentLogs, 2)
	childLogs, err := model.GetLogByTokenId(child.Id)
	require.NoError(t, err)
	require.Len(t, childLogs, 1)
	require.Equal(t, child.Id, childLogs[0].TokenId)
}

func TestDeleteParentTokenDeletesSubTokens(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	parent := seedToken(t, db, 1, "parent", "parent-key")
	child, err := model.CreateSubToken(parent.Id, parent.UserId, "child")
	require.NoError(t, err)
	require.NoError(t, model.DeleteTokenById(parent.Id, parent.UserId))
	var stored model.Token
	require.Error(t, db.First(&stored, child.Id).Error)
	require.NoError(t, db.Unscoped().First(&stored, child.Id).Error)
	require.NotZero(t, stored.DeletedAt.Valid)
}

func TestCreateSubTokenByApiKeyAllowsExhaustedParent(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	parent := seedToken(t, db, 1, "parent", "parent-key")
	parent.Status = common.TokenStatusExhausted
	parent.UnlimitedQuota = false
	parent.RemainQuota = 0
	require.NoError(t, db.Save(parent).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/subkeys", map[string]string{"name": "automation"}, 1)
	ctx.Set("token_id", parent.Id)
	CreateSubTokenByApiKey(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success)
	var created struct {
		Key string `json:"key"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &created))
	require.NotEmpty(t, created.Key)
}
