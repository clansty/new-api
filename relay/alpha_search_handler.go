package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"
)

func AlphaSearchHelper(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	info.InitChannelMeta(c)
	request, ok := info.Request.(*dto.AlphaSearchRequest)
	if !ok {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type, expected dto.AlphaSearchRequest, got %T", info.Request),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	switch info.ApiType {
	case constant.APITypeOpenAI, constant.APITypeCodex, constant.APITypePassThrough:
	default:
		return types.NewError(fmt.Errorf("alpha search is unsupported for api type %d", info.ApiType), types.ErrorCodeInvalidApiType)
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType)
	}
	adaptor.Init(info)

	requestBody, err := prepareAlphaSearchRequestBody(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	response, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	httpResponse, ok := response.(*http.Response)
	if !ok || httpResponse == nil {
		return types.NewError(fmt.Errorf("invalid alpha search response type %T", response), types.ErrorCodeBadResponseBody)
	}

	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		apiError := service.RelayErrorHandler(c.Request.Context(), httpResponse, false)
		service.ResetStatusCode(apiError, c.GetString("status_code_mapping"))
		return apiError
	}

	responseBody, err := io.ReadAll(httpResponse.Body)
	service.CloseResponseBodyGracefully(httpResponse)
	if err != nil {
		return types.NewError(err, types.ErrorCodeReadResponseBodyFailed)
	}
	service.IOCopyBytesGracefully(c, httpResponse, responseBody)
	service.PostTextConsumeQuota(c, info, &dto.Usage{}, nil)
	return nil
}

func prepareAlphaSearchRequestBody(c *gin.Context, info *relaycommon.RelayInfo, request *dto.AlphaSearchRequest) (io.Reader, error) {
	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return nil, fmt.Errorf("map alpha search model: %w", err)
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, fmt.Errorf("read alpha search body: %w", err)
	}
	body, err := storage.Bytes()
	if err != nil {
		return nil, fmt.Errorf("read alpha search body bytes: %w", err)
	}
	body, err = sjson.SetBytes(body, "model", request.Model)
	if err != nil {
		return nil, fmt.Errorf("set alpha search model: %w", err)
	}
	if len(info.ParamOverride) > 0 {
		body, err = relaycommon.ApplyParamOverrideWithRelayInfo(body, info)
		if err != nil {
			return nil, fmt.Errorf("apply alpha search parameter override: %w", err)
		}
	}
	return bytes.NewReader(body), nil
}
