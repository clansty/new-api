package helper

import (
	"context"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
)

func newStreamScheduler(ctx context.Context, wg *sync.WaitGroup, writeMutex *sync.Mutex) streamScheduler {
	return func(delay time.Duration, handler func()) {
		wg.Add(1)
		gopool.Go(func() {
			defer wg.Done()

			timer := time.NewTimer(delay)
			defer timer.Stop()

			select {
			case <-timer.C:
				writeMutex.Lock()
				defer writeMutex.Unlock()
				handler()
			case <-ctx.Done():
			}
		})
	}
}
