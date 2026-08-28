package model

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRestrictRetryErrorLogsToAdmin_whenRetryEventuallySucceeds(t *testing.T) {
	// Given: 同一用户的一次请求先失败后成功，且存在不相关的错误日志。
	truncateTables(t)
	const (
		userID    = 101
		requestID = "retry-success"
	)
	retryError := Log{UserId: userID, Type: LogTypeError, RequestId: requestID}
	success := Log{UserId: userID, Type: LogTypeConsume, RequestId: requestID}
	otherRequestError := Log{UserId: userID, Type: LogTypeError, RequestId: "final-failure"}
	otherUserError := Log{UserId: userID + 1, Type: LogTypeError, RequestId: requestID}
	require.NoError(t, LOG_DB.Create(&[]*Log{&retryError, &success, &otherRequestError, &otherUserError}).Error)

	// When: 成功重试结束后收紧此前错误日志的可见性。
	err := RestrictRetryErrorLogsToAdmin(userID, requestID)

	// Then: 只隐藏同用户、同请求的错误尝试，成功日志及其他失败保持可见。
	require.NoError(t, err)
	var storedRetry, storedSuccess, storedOtherRequest, storedOtherUser Log
	require.NoError(t, LOG_DB.First(&storedRetry, retryError.Id).Error)
	require.NoError(t, LOG_DB.First(&storedSuccess, success.Id).Error)
	require.NoError(t, LOG_DB.First(&storedOtherRequest, otherRequestError.Id).Error)
	require.NoError(t, LOG_DB.First(&storedOtherUser, otherUserError.Id).Error)
	require.False(t, storedRetry.UserVisible)
	require.True(t, storedSuccess.UserVisible)
	require.True(t, storedOtherRequest.UserVisible)
	require.True(t, storedOtherUser.UserVisible)
}

func TestRestrictRetryErrorLogsToAdmin_whenRequestIDIsEmpty(t *testing.T) {
	// Given: 历史日志中可能存在空请求 ID。
	truncateTables(t)
	log := Log{UserId: 102, Type: LogTypeError, RequestId: ""}
	require.NoError(t, LOG_DB.Create(&log).Error)

	// When: 调用方缺少可用于精确关联的请求 ID。
	err := RestrictRetryErrorLogsToAdmin(log.UserId, "")

	// Then: 不得误伤其他空请求 ID 日志。
	require.NoError(t, err)
	var stored Log
	require.NoError(t, LOG_DB.First(&stored, log.Id).Error)
	require.True(t, stored.UserVisible)
}

func TestRestrictSuccessfulRetryErrorLogs_whenSameChannelRetried(t *testing.T) {
	// Given: 同一渠道原地重试成功，渠道 ID 虽相同但尝试次数为两次。
	truncateTables(t)
	const (
		userID    = 105
		requestID = "in-place-retry"
	)
	log := Log{UserId: userID, Type: LogTypeError, RequestId: requestID}
	require.NoError(t, LOG_DB.Create(&log).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(common.RequestIdKey, requestID)
	c.Set("use_channel", []string{"7", "7"})
	common.SetContextKey(c, constant.ContextKeyAutoRetryAttempted, true)

	// When: 最终成功路径处理重试日志。
	err := RestrictSuccessfulRetryErrorLogs(c, userID)

	// Then: 重复的同一渠道也被识别为真实重试。
	require.NoError(t, err)
	var stored Log
	require.NoError(t, LOG_DB.First(&stored, log.Id).Error)
	require.False(t, stored.UserVisible)
}

func TestRestrictSuccessfulRetryErrorLogs_whenRequestDidNotRetry(t *testing.T) {
	// Given: 请求只有一次渠道尝试，即使存在同请求错误日志也不能推断为成功重试。
	truncateTables(t)
	const (
		userID    = 106
		requestID = "single-attempt"
	)
	log := Log{UserId: userID, Type: LogTypeError, RequestId: requestID}
	require.NoError(t, LOG_DB.Create(&log).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(common.RequestIdKey, requestID)
	c.Set("use_channel", []string{"7", "8"})

	// When: 成功路径检查是否发生过自动重试。
	err := RestrictSuccessfulRetryErrorLogs(c, userID)

	// Then: 单次尝试的错误日志保持用户可见。
	require.NoError(t, err)
	var stored Log
	require.NoError(t, LOG_DB.First(&stored, log.Id).Error)
	require.True(t, stored.UserVisible)
}

func TestGetUserLogs_HidesAdminOnlyRetryErrors(t *testing.T) {
	// Given: 成功重试的失败尝试已限制为管理员可见，最终成功日志仍可见。
	truncateTables(t)
	const (
		userID    = 103
		requestID = "retry-query"
	)
	require.NoError(t, LOG_DB.Create(&[]Log{
		{UserId: userID, Type: LogTypeError, RequestId: requestID},
		{UserId: userID, Type: LogTypeConsume, RequestId: requestID},
	}).Error)
	require.NoError(t, RestrictRetryErrorLogsToAdmin(userID, requestID))

	// When: 普通用户和管理员分别查询日志。
	userLogs, userTotal, userErr := GetUserLogs(userID, LogTypeUnknown, 0, 0, "", "", 0, 10, "", "")
	adminLogs, adminTotal, adminErr := GetAllLogs(LogTypeUnknown, 0, 0, "", "", "", 0, 10, 0, "", "")

	// Then: 用户分页与总数都排除失败尝试，管理员仍看到完整重试链。
	require.NoError(t, userErr)
	require.Equal(t, int64(1), userTotal)
	require.Len(t, userLogs, 1)
	require.Equal(t, LogTypeConsume, userLogs[0].Type)
	require.NoError(t, adminErr)
	require.Equal(t, int64(2), adminTotal)
	require.Len(t, adminLogs, 2)
}

func TestGetLogByTokenId_HidesAdminOnlyRetryErrors(t *testing.T) {
	// Given: 令牌日志中包含一次成功重试留下的失败尝试。
	truncateTables(t)
	const (
		tokenID   = 204
		userID    = 104
		requestID = "token-retry"
	)
	require.NoError(t, LOG_DB.Create(&[]Log{
		{UserId: userID, TokenId: tokenID, Type: LogTypeError, RequestId: requestID},
		{UserId: userID, TokenId: tokenID, Type: LogTypeConsume, RequestId: requestID},
	}).Error)
	require.NoError(t, RestrictRetryErrorLogsToAdmin(userID, requestID))

	// When: 使用令牌身份查询近期日志。
	logs, err := GetLogByTokenId(tokenID)

	// Then: 令牌持有者只看到最终成功日志。
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, LogTypeConsume, logs[0].Type)
}

func TestGetUserLogs_PreservesFinalFailure(t *testing.T) {
	// Given: 请求最终失败，没有成功路径收紧其错误日志可见性。
	truncateTables(t)
	const userID = 107
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    userID,
		Type:      LogTypeError,
		RequestId: "final-failure",
	}).Error)

	// When: 用户查询自己的使用日志。
	logs, total, err := GetUserLogs(userID, LogTypeUnknown, 0, 0, "", "", 0, 10, "", "")

	// Then: 最终失败仍对用户可见，便于排查真实失败请求。
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	require.Equal(t, LogTypeError, logs[0].Type)
}
