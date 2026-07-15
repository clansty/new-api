package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const maxPerformanceRangeSeconds = int64(31 * 24 * time.Hour / time.Second)

func GetLogPerformance(c *gin.Context) {
	endTimestamp, err := parsePerformanceTimestamp(c.Query("end_timestamp"), time.Now().Unix())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "结束时间格式不正确"})
		return
	}
	startTimestamp, err := parsePerformanceTimestamp(c.Query("start_timestamp"), endTimestamp-int64(24*time.Hour/time.Second))
	if err != nil || startTimestamp >= endTimestamp || endTimestamp-startTimestamp > maxPerformanceRangeSeconds {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "时间范围必须在 31 天以内"})
		return
	}
	channelID, err := parsePerformanceChannel(c.Query("channel"))
	if err != nil || channelID < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "渠道参数不正确"})
		return
	}
	groupBy := model.PerformanceGroup(c.DefaultQuery("group_by", string(model.PerformanceGroupModel)))
	if groupBy != model.PerformanceGroupModel && groupBy != model.PerformanceGroupChannel {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "分组参数不正确"})
		return
	}
	result, err := model.GetLogPerformance(model.LogPerformanceQuery{
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ModelName:      c.Query("model_name"),
		ChannelID:      channelID,
		GroupBy:        groupBy,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func parsePerformanceTimestamp(value string, fallback int64) (int64, error) {
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func parsePerformanceChannel(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}
