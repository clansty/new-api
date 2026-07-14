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

func (a *Adaptor) ConvertJSONImageRequest(request dto.ImageRequest) dto.ImageRequest {
	return request
}

func (a *Adaptor) DoImageResponse(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if info.IsStream {
		return openaiImageStreamHandler(c, info, resp)
	}
	return OpenaiHandlerWithUsage(c, info, resp)
}

func openaiImageStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return openaiImageJSONAsStreamHandler(c, info, resp)
	}
	defer service.CloseResponseBodyGracefully(resp)

	usage := &dto.Usage{}
	var lastData []byte
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
					if strings.EqualFold(currentEvent, "error") || eventType == "error" || eventType == "upstream_error" || len(event.Error) > 0 {
						info.StreamStatus.RecordError("upstream image stream returned error event")
					}
				}

				var response dto.SimpleResponse
				if err := common.Unmarshal(lastData, &response); err == nil {
					normalizeImageUsage(&response.Usage)
					if service.ValidUsage(&response.Usage) {
						usage = &response.Usage
					}
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
		return nil, types.WithOpenAIError(*openAIError, resp.StatusCode)
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
