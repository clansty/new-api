package helper

import (
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

type streamScheduler func(delay time.Duration, handler func())

// StreamResult is passed to each dataHandler invocation, providing methods
// to record soft errors, signal fatal stops, or mark normal completion.
// StreamScannerHandler checks IsStopped() after each callback invocation.
type StreamResult struct {
	status    *relaycommon.StreamStatus
	stopped   bool
	scheduler streamScheduler
}

func newStreamResult(status *relaycommon.StreamStatus, scheduler streamScheduler) *StreamResult {
	return &StreamResult{status: status, scheduler: scheduler}
}

// Schedule 让延时写入与当前流的其他写操作串行，避免并发操作 ResponseWriter。
func (r *StreamResult) Schedule(delay time.Duration, handler func()) {
	r.scheduler(delay, handler)
}

// Error records a soft error. The stream continues processing.
// Can be called multiple times per chunk.
func (r *StreamResult) Error(err error) {
	if err == nil {
		return
	}
	r.status.RecordError(err.Error())
}

// Stop records a fatal error and marks the stream to stop after this chunk.
func (r *StreamResult) Stop(err error) {
	if err != nil {
		r.status.RecordError(err.Error())
	}
	r.status.SetEndReason(relaycommon.StreamEndReasonHandlerStop, err)
	r.stopped = true
}

// Done signals that the handler has finished processing normally
// (e.g., Dify "message_end"). The stream stops after this chunk.
func (r *StreamResult) Done() {
	r.status.SetEndReason(relaycommon.StreamEndReasonDone, nil)
	r.stopped = true
}

// IsStopped returns whether Stop() or Done() was called during this chunk.
func (r *StreamResult) IsStopped() bool {
	return r.stopped
}

// reset clears the per-chunk stopped flag so the object can be reused.
func (r *StreamResult) reset() {
	r.stopped = false
}
