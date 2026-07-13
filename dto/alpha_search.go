package dto

type AlphaSearchRequest struct {
	BaseRequest
	Model string `json:"model"`
}

func (r *AlphaSearchRequest) SetModelName(modelName string) {
	r.Model = modelName
}
