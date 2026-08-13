package dto

import "encoding/json"

type AlphaSearchRequest struct {
	BaseRequest
	ID              string          `json:"id,omitempty"`
	Model           string          `json:"model"`
	Input           string          `json:"input,omitempty"`
	Commands        json.RawMessage `json:"commands,omitempty"`
	Settings        json.RawMessage `json:"settings,omitempty"`
	MaxOutputTokens *uint           `json:"max_output_tokens,omitempty"`
}

func (r *AlphaSearchRequest) SetModelName(modelName string) {
	r.Model = modelName
}
