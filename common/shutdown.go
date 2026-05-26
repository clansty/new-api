package common

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
)

// 优雅退出协调器
//
// 设计目标:
//   - 收到 SIGTERM/SIGINT 后, 让 /api/status 立刻返回 503, 使 Docker Swarm /
//     LB 把容器标记为 unhealthy 并停止转发新流量;
//   - 后台 goroutine 通过 ShutdownCtx() 监听取消, 自然退出;
//   - 关键 fire-and-forget goroutine 用 GoTracked 注册到 BackgroundWG,
//     主流程在关闭 DB/Redis 前 WaitBackground, 防止数据写入后被强杀.

var (
	shuttingDown   atomic.Bool
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	backgroundWG   sync.WaitGroup
)

func init() {
	shutdownCtx, shutdownCancel = context.WithCancel(context.Background())
}

// IsShuttingDown 返回进程是否正在关闭.
// /api/status 等健康检查端点应在第一时间检查此标志并返回 503.
// 长任务的循环内部也应定期 check 以便提前退出.
func IsShuttingDown() bool {
	return shuttingDown.Load()
}

// ShutdownCtx 返回一个在收到关闭信号时会被取消的 context.
// 所有长期运行的后台 goroutine 都应在 select 中监听 ShutdownCtx().Done()
// 以便优雅退出.
func ShutdownCtx() context.Context {
	return shutdownCtx
}

// TriggerShutdown 标记进程进入关闭状态并取消 ShutdownCtx.
// 仅由 main 包的 signal handler 调用. 幂等.
func TriggerShutdown() {
	if shuttingDown.CompareAndSwap(false, true) {
		shutdownCancel()
	}
}

// SleepOrDone 睡眠 d, 期间 ctx 取消则提前返回 false; 正常 tick 到时返回 true.
// 后台循环常用 idiom, 替代 time.Sleep + 单独 select.
func SleepOrDone(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// GoTracked 用 gopool 启动一个被 BackgroundWG 追踪的 goroutine.
// 适用于"HTTP handler 派生但需要在关闭前完成"的写库/写缓存任务,
// 例如 channel 自动测试. graceful shutdown 会在 close DB/Redis 前 WaitBackground.
func GoTracked(fn func()) {
	backgroundWG.Add(1)
	gopool.Go(func() {
		defer backgroundWG.Done()
		fn()
	})
}

// WaitBackground 等待所有 GoTracked 派生的 goroutine 完成, 最多等 timeout.
// 超时仅 SysLog, 不阻塞 shutdown 序列继续推进.
func WaitBackground(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		backgroundWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		SysError("graceful shutdown: background tasks did not finish within " + timeout.String())
	}
}
