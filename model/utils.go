package model

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

const (
	BatchUpdateTypeUserQuota = iota
	BatchUpdateTypeTokenQuota
	BatchUpdateTypeTokenUsedQuota
	BatchUpdateTypeUsedQuota
	BatchUpdateTypeChannelUsedQuota
	BatchUpdateTypeRequestCount
	BatchUpdateTypeCount // if you add a new type, you need to add a new map and a new lock
)

var batchUpdateStores []map[int]int
var batchUpdateLocks []sync.Mutex

// batchUpdaterCtx 独立于 common.ShutdownCtx, 让 BatchUpdater 在 HTTP drain 期间
// 继续 tick 写库, 缩短崩溃时丢失数据的窗口.
// 由 main 在 HTTP server 关闭后通过 StopBatchUpdater 显式取消.
var (
	batchUpdaterCtx    context.Context
	batchUpdaterCancel context.CancelFunc
)

func init() {
	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateStores = append(batchUpdateStores, make(map[int]int))
		batchUpdateLocks = append(batchUpdateLocks, sync.Mutex{})
	}
	batchUpdaterCtx, batchUpdaterCancel = context.WithCancel(context.Background())
}

func InitBatchUpdater() {
	gopool.Go(func() {
		ticker := time.NewTicker(time.Duration(common.BatchUpdateInterval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-batchUpdaterCtx.Done():
				return
			case <-ticker.C:
				if err := batchUpdate(); err != nil {
					common.SysError("batch update tick failed: " + err.Error())
				}
			}
		}
	})
}

// StopBatchUpdater 停止后台 BatchUpdater goroutine.
// 必须由 main 在所有可能写入 batch 的 goroutine 都结束后调用,
// 之后再执行最终 FlushBatchUpdater.
func StopBatchUpdater() {
	batchUpdaterCancel()
}

// FlushBatchUpdater 同步 flush 所有内存中的额度/计数增量, 失败的 delta 会被回填到 map.
// 返回最后一次 batchUpdate 的错误; 调用方应在 graceful shutdown 中带 deadline 重试.
func FlushBatchUpdater() error {
	return batchUpdate()
}

func addNewRecord(type_ int, id int, value int) {
	batchUpdateLocks[type_].Lock()
	defer batchUpdateLocks[type_].Unlock()
	batchUpdateStores[type_][id] += value
}

// reenqueueRecord 把因 DB 写入失败的 delta 重新加回 map, 用于 batchUpdate 出错回滚.
// 加法的方式合并: 即使 tick 间隔内又有新写入也不会覆盖.
func reenqueueRecord(type_ int, id int, value int) {
	batchUpdateLocks[type_].Lock()
	defer batchUpdateLocks[type_].Unlock()
	batchUpdateStores[type_][id] += value
}

// batchUpdate 把所有 batch map 写入数据库; 单个 key 写失败时把该 delta 加回 map,
// 不会丢数据. 整体若有任何失败, 返回最后一个错误.
func batchUpdate() error {
	hasData := false
	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateLocks[i].Lock()
		if len(batchUpdateStores[i]) > 0 {
			hasData = true
		}
		batchUpdateLocks[i].Unlock()
		if hasData {
			break
		}
	}
	if !hasData {
		return nil
	}

	common.SysLog("batch update started")
	var lastErr error
	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateLocks[i].Lock()
		store := batchUpdateStores[i]
		batchUpdateStores[i] = make(map[int]int)
		batchUpdateLocks[i].Unlock()

		for key, value := range store {
			var err error
			switch i {
			case BatchUpdateTypeUserQuota:
				err = increaseUserQuota(key, value)
			case BatchUpdateTypeTokenQuota:
				err = increaseTokenQuota(key, value)
			case BatchUpdateTypeTokenUsedQuota:
				err = updateTokenUsedQuota(key, value)
			case BatchUpdateTypeUsedQuota:
				updateUserUsedQuota(key, value)
			case BatchUpdateTypeRequestCount:
				updateUserRequestCount(key, value)
			case BatchUpdateTypeChannelUsedQuota:
				updateChannelUsedQuota(key, value)
			}
			if err != nil {
				common.SysError("batch update failed (type=" + batchUpdateTypeName(i) + "), requeueing delta: " + err.Error())
				reenqueueRecord(i, key, value)
				lastErr = err
			}
		}
	}
	common.SysLog("batch update finished")
	return lastErr
}

func batchUpdateTypeName(t int) string {
	switch t {
	case BatchUpdateTypeUserQuota:
		return "user_quota"
	case BatchUpdateTypeTokenQuota:
		return "token_quota"
	case BatchUpdateTypeTokenUsedQuota:
		return "token_used_quota"
	case BatchUpdateTypeUsedQuota:
		return "used_quota"
	case BatchUpdateTypeChannelUsedQuota:
		return "channel_used_quota"
	case BatchUpdateTypeRequestCount:
		return "request_count"
	default:
		return "unknown"
	}
}

func RecordExist(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func shouldUpdateRedis(fromDB bool, err error) bool {
	return common.RedisEnabled && fromDB && err == nil
}
