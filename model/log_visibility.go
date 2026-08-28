package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

func RestrictRetryErrorLogsToAdmin(userID int, requestID string) error {
	if userID <= 0 || requestID == "" {
		return nil
	}
	return LOG_DB.Model(&Log{}).
		Where("user_id = ? AND request_id = ? AND type = ?", userID, requestID, LogTypeError).
		Update("user_visible", false).Error
}

func RestrictSuccessfulRetryErrorLogs(c *gin.Context, userID int) error {
	if !common.GetContextKeyBool(c, constant.ContextKeyAutoRetryAttempted) {
		return nil
	}
	return RestrictRetryErrorLogsToAdmin(userID, c.GetString(common.RequestIdKey))
}
