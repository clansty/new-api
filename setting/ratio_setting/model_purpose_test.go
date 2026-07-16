package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelPurpose_whenModelHasExplicitPurposes(t *testing.T) {
	previous := ModelPurpose2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateModelPurposeByJSONString(previous))
	})

	require.NoError(t, UpdateModelPurposeByJSONString(`{"gpt-image-2":["image"]}`))

	require.True(t, IsModelPurposeAllowed("gpt-image-2", ModelPurposeImage))
	require.False(t, IsModelPurposeAllowed("gpt-image-2", ModelPurposeChat))
	require.True(t, IsModelPurposeAllowed("unconfigured-model", ModelPurposeChat))
}

func TestUpdateModelPurposeByJSONString_whenPurposeIsInvalid(t *testing.T) {
	previous := ModelPurpose2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateModelPurposeByJSONString(previous))
	})

	err := UpdateModelPurposeByJSONString(`{"model-a":["not-a-purpose"]}`)

	require.Error(t, err)
	require.Equal(t, previous, ModelPurpose2JSONString())
}

func TestInitRatioSettings_whenUsingBuiltInImageModel(t *testing.T) {
	previous := ModelPurpose2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateModelPurposeByJSONString(previous))
	})

	modelPurposeMap.Clear()
	InitRatioSettings()

	require.True(t, IsModelPurposeAllowed("gpt-image-2", ModelPurposeImage))
	require.False(t, IsModelPurposeAllowed("gpt-image-2", ModelPurposeChat))
}

func TestFilterUncoveredModels_doesNotRemovePurposeConfiguration(t *testing.T) {
	previous := ModelPurpose2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateModelPurposeByJSONString(previous))
	})
	require.NoError(t, UpdateModelPurposeByJSONString(`{"kept-model":["chat"],"missing-model":["image"]}`))

	changed, _, _ := FilterUncoveredModels([]string{"kept-model"})

	require.NotContains(t, changed, "ModelPurpose")
	require.JSONEq(t, `{"kept-model":["chat"],"missing-model":["image"]}`, ModelPurpose2JSONString())
}
