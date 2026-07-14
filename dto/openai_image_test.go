package dto

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestImageRequestJSON_whenImageEditUsesCurrentFields(t *testing.T) {
	var request ImageRequest
	require.NoError(t, common.Unmarshal([]byte(`{
		"model":"gpt-image-1",
		"prompt":"replace the background",
		"images":[{"image_url":"https://example.com/source.png"}],
		"mask":{"file_id":"file-mask"},
		"input_fidelity":"high",
		"output_compression":0,
		"partial_images":0,
		"stream":false
	}`), &request))

	require.NotNil(t, request.Stream)
	require.False(t, request.IsStream(nil))

	encoded, err := common.Marshal(request)
	require.NoError(t, err)

	var fields map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(encoded, &fields))
	require.JSONEq(t, `[{"image_url":"https://example.com/source.png"}]`, string(fields["images"]))
	require.JSONEq(t, `{"file_id":"file-mask"}`, string(fields["mask"]))
	require.JSONEq(t, `"high"`, string(fields["input_fidelity"]))
	require.Equal(t, "0", string(fields["output_compression"]))
	require.Equal(t, "0", string(fields["partial_images"]))
	require.Equal(t, "false", string(fields["stream"]))
}

func TestImageRequestJSON_whenStreamIsAbsent(t *testing.T) {
	var request ImageRequest
	require.NoError(t, common.Unmarshal([]byte(`{"model":"gpt-image-1","prompt":"draw a cat"}`), &request))

	require.Nil(t, request.Stream)
	require.False(t, request.IsStream(nil))
}
