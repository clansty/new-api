package openai

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type imageStreamCompletedEvent struct {
	Type          string     `json:"type"`
	CreatedAt     int64      `json:"created_at"`
	URL           string     `json:"url,omitempty"`
	B64JSON       string     `json:"b64_json,omitempty"`
	RevisedPrompt string     `json:"revised_prompt,omitempty"`
	Usage         *dto.Usage `json:"usage,omitempty"`
}

// imageOutputProbe 用于识别上游到底有没有吐图。上游经常以 200 返回空 data
// 或只带文字/错误说明的 completed 事件，若不识别就会被 ImageHelper 的
// PromptTokens 兜底当成一次成功调用扣费（表现为“输入 1、输出 0”）。
type imageOutputProbe struct {
	Type    string `json:"type"`
	URL     string `json:"url"`
	B64JSON string `json:"b64_json"`
	Result  string `json:"result"`
}

type imageOutputEnvelope struct {
	imageOutputProbe
	Data     []imageOutputProbe   `json:"data"`
	Output   []imageOutputProbe   `json:"output"`
	Item     *imageOutputProbe    `json:"item"`
	Response *imageOutputEnvelope `json:"response"`
}

func (p imageOutputProbe) hasImage() bool {
	return strings.TrimSpace(p.URL) != "" ||
		strings.TrimSpace(p.B64JSON) != "" ||
		strings.TrimSpace(p.Result) != ""
}

func (p imageOutputEnvelope) hasImage() bool {
	if p.imageOutputProbe.hasImage() {
		return true
	}
	for _, item := range p.Data {
		if item.hasImage() {
			return true
		}
	}
	for _, item := range p.Output {
		if item.hasImage() {
			return true
		}
	}
	if p.Item != nil && p.Item.hasImage() {
		return true
	}
	return false
}

func (a *Adaptor) ConvertJSONImageRequest(request dto.ImageRequest) dto.ImageRequest {
	return request
}

func (a *Adaptor) DoImageResponse(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if info.IsStream {
		return openaiImageStreamHandler(c, info, resp)
	}
	return openaiImageJSONHandler(c, info, resp)
}

// openaiImageJSONHandler 与 OpenaiHandlerWithUsage 的区别：先确认响应确实
// 带有图片或明确的 usage，再写回客户端；空响应 / 内嵌 error 都按上游失败处理，
// 避免未产出图片的请求被结算。
func openaiImageJSONHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var usageResp dto.SimpleResponse
	if err := common.Unmarshal(responseBody, &usageResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	// 部分上游会用 200 携带 error 字段，此时既不能转发成功，也不能计费
	if openAIError := usageResp.GetOpenAIError(); openAIError != nil && openAIError.Type != "" {
		return nil, types.WithOpenAIError(*openAIError, upstreamImageErrorStatus(resp.StatusCode, openAIError))
	}

	if !imageResponseHasOutput(responseBody) {
		return nil, newImageNoOutputError(responseBody)
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)
	normalizeImageUsage(&usageResp.Usage)
	applyUsagePostProcessing(info, &usageResp.Usage, responseBody)
	return &usageResp.Usage, nil
}

func openaiImageStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return openaiImageJSONAsStreamHandler(c, info, resp)
	}
	defer service.CloseResponseBodyGracefully(resp)

	usage := &dto.Usage{}
	var lastData []byte
	hasImageOutput := false
	streamErrorMessage := ""
	info.StreamStatus = relaycommon.NewStreamStatus()
	helper.SetEventStreamHeaders(c)

	reader := bufio.NewReader(resp.Body)
	currentEvent := ""
	for {
		line, readErr := reader.ReadString('\n')
		trimmed := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if strings.HasPrefix(trimmed, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
		} else if strings.HasPrefix(trimmed, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if data == "[DONE]" {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
			} else if data != "" {
				info.SetFirstResponseTime()
				info.ReceivedResponseCount++
				lastData = common.StringToByteSlice(data)

				var event struct {
					Type  string          `json:"type"`
					Error json.RawMessage `json:"error"`
				}
				if err := common.Unmarshal(lastData, &event); err == nil {
					eventType := strings.ToLower(strings.TrimSpace(event.Type))
					// "error": null 是正常事件的常见字段，只有非空错误体才算失败
					errorPayload := strings.TrimSpace(string(event.Error))
					hasErrorPayload := errorPayload != "" && errorPayload != "null"
					if strings.EqualFold(currentEvent, "error") || eventType == "error" || eventType == "upstream_error" ||
						eventType == "response.failed" || eventType == "response.incomplete" || hasErrorPayload {
						streamErrorMessage = "上游 image 流返回了错误事件"
						info.StreamStatus.RecordError("upstream image stream returned error event")
					}
				}

				if imageStreamDataHasOutput(lastData) {
					hasImageOutput = true
				}

				if streamUsage := imageStreamUsage(lastData); streamUsage != nil {
					usage = streamUsage
				}
			}
		} else if trimmed == "" {
			currentEvent = ""
		}

		if line != "" {
			if _, err := c.Writer.Write([]byte(line)); err != nil {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, err)
				return usage, nil
			}
			if trimmed == "" {
				if err := helper.FlushWriter(c); err != nil {
					info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, err)
					return usage, nil
				}
			}
		}

		if readErr != nil {
			if readErr != io.EOF {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonScannerErr, readErr)
			} else if info.StreamStatus.EndReason == relaycommon.StreamEndReasonNone {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonEOF, nil)
			}
			break
		}
	}

	_ = helper.FlushWriter(c)
	applyUsagePostProcessing(info, usage, lastData)

	if streamErrorMessage != "" || !hasImageOutput {
		message := streamErrorMessage
		if message == "" {
			message = "上游 image 流结束但没有生成任何图片"
		}
		info.StreamStatus.RecordError(message)
		return usage, newImageNoOutputErrorWithMessage(message, info.ReceivedResponseCount > 0)
	}
	return usage, nil
}

func openaiImageJSONAsStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var imageResponse dto.ImageResponse
	if err := common.Unmarshal(body, &imageResponse); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	var response dto.SimpleResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if openAIError := response.GetOpenAIError(); openAIError != nil && openAIError.Type != "" {
		return nil, types.WithOpenAIError(*openAIError, upstreamImageErrorStatus(resp.StatusCode, openAIError))
	}
	if !imageResponseHasOutput(body) {
		return nil, newImageNoOutputError(body)
	}
	normalizeImageUsage(&response.Usage)
	applyUsagePostProcessing(info, &response.Usage, body)

	helper.SetEventStreamHeaders(c)
	c.Status(http.StatusOK)
	created := imageResponse.Created
	if created == 0 {
		created = time.Now().Unix()
	}
	eventName := "image_generation.completed"
	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		eventName = "image_edit.completed"
	}
	for _, image := range imageResponse.Data {
		payload := imageStreamCompletedEvent{
			Type:          eventName,
			CreatedAt:     created,
			URL:           image.Url,
			B64JSON:       image.B64Json,
			RevisedPrompt: image.RevisedPrompt,
		}
		if service.ValidUsage(&response.Usage) {
			payload.Usage = &response.Usage
		}
		data, err := common.Marshal(payload)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventName, data); err != nil {
			return &response.Usage, nil
		}
		_ = helper.FlushWriter(c)
	}
	helper.Done(c)
	info.ReceivedResponseCount += len(imageResponse.Data)
	if info.StreamStatus == nil {
		info.StreamStatus = relaycommon.NewStreamStatus()
	}
	info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
	return &response.Usage, nil
}

// imageStreamUsage 兼容两种事件形态：images 风格把 usage 放在顶层，
// responses 风格放在 response.usage 下；否则会退回 ImageHelper 的兜底 token。
func imageStreamUsage(data []byte) *dto.Usage {
	var event struct {
		Usage    *dto.Usage `json:"usage"`
		Response *struct {
			Usage *dto.Usage `json:"usage"`
		} `json:"response"`
	}
	if err := common.Unmarshal(data, &event); err != nil {
		return nil
	}
	candidate := event.Usage
	if !service.ValidUsage(candidate) && event.Response != nil {
		candidate = event.Response.Usage
	}
	if candidate == nil {
		return nil
	}
	normalizeImageUsage(candidate)
	if !service.ValidUsage(candidate) {
		return nil
	}
	return candidate
}

func imageResponseHasOutput(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var envelope imageOutputEnvelope
	if err := common.Unmarshal(body, &envelope); err != nil {
		return false
	}
	if envelope.hasImage() {
		return true
	}
	if envelope.Response != nil {
		return envelope.Response.hasImage()
	}
	return false
}

// imageStreamDataHasOutput 忽略 partial_image 事件：半张图不能证明生成完成，
// 否则上游中途失败时仍会被结算。
func imageStreamDataHasOutput(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	var envelope imageOutputEnvelope
	if err := common.Unmarshal(data, &envelope); err != nil {
		return false
	}
	if strings.Contains(strings.ToLower(envelope.Type), "partial_image") {
		return false
	}
	if envelope.hasImage() {
		return true
	}
	if envelope.Response != nil {
		return envelope.Response.hasImage()
	}
	return false
}

// upstreamImageErrorStatus 把 200 的“伪成功错误”映射为可重试的上游错误码，
// 否则 shouldRetry 会因 2xx 状态码直接放弃重试；内容审核类拒绝重试无意义，
// 保持 400 让客户端按客户端错误处理。
func upstreamImageErrorStatus(statusCode int, openAIError *types.OpenAIError) int {
	if statusCode >= 200 && statusCode < 300 {
		if openAIError != nil && isImageContentPolicySignal(openAIError.Type+" "+fmt.Sprintf("%v", openAIError.Code)+" "+openAIError.Message) {
			return http.StatusBadRequest
		}
		return http.StatusBadGateway
	}
	return statusCode
}

func isImageContentPolicySignal(text string) bool {
	lower := strings.ToLower(text)
	for _, marker := range []string{
		"content policy", "content_policy", "content filter", "content_filter",
		"safety system", "safety policy", "safety violation", "moderation",
		"安全系统", "安全策略", "安全政策", "内容政策", "内容审核", "违规内容", "不适合生成",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func newImageNoOutputError(body []byte) *types.NewAPIError {
	snippet := strings.TrimSpace(string(body))
	const maxSnippet = 512
	if len(snippet) > maxSnippet {
		snippet = snippet[:maxSnippet] + "...(truncated)"
	}
	// 模型以文字拒绝了生成（内容审核），属于客户端错误，不应重试也不应计费
	if isImageContentPolicySignal(snippet) {
		return types.NewOpenAIError(fmt.Errorf("上游未生成图片: %s", snippet), types.ErrorCodeEmptyResponse, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	message := "上游返回成功但没有生成任何图片"
	if snippet != "" {
		message = fmt.Sprintf("%s: %s", message, snippet)
	}
	return newImageNoOutputErrorWithMessage(message, false)
}

func newImageNoOutputErrorWithMessage(message string, written bool) *types.NewAPIError {
	var options []types.NewAPIErrorOptions
	if written {
		options = append(options, types.ErrOptionWithSkipRetry())
	}
	return types.NewOpenAIError(fmt.Errorf("%s", message), types.ErrorCodeEmptyResponse, http.StatusBadGateway, options...)
}

func normalizeImageUsage(usage *dto.Usage) {
	if usage.InputTokens != 0 {
		usage.PromptTokens = usage.InputTokens
	}
	if usage.OutputTokens != 0 {
		usage.CompletionTokens = usage.OutputTokens
	}
	if usage.InputTokensDetails != nil {
		usage.PromptTokensDetails = *usage.InputTokensDetails
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
}
