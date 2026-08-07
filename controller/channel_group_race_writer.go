package controller

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

var errChannelRaceLost = errors.New("渠道组竞速已由其他成员获胜")

type channelRaceResponseWriter struct {
	ctx        context.Context
	downstream gin.ResponseWriter
	header     http.Header
	status     int
	size       int
	onReady    func()
	readyOnce  sync.Once
	decideOnce sync.Once
	decided    chan struct{}
	winner     atomic.Bool
	writeMu    sync.Mutex
	committed  bool
}

func newChannelRaceResponseWriter(ctx context.Context, downstream gin.ResponseWriter, onReady func()) *channelRaceResponseWriter {
	return &channelRaceResponseWriter{
		ctx:        ctx,
		downstream: downstream,
		header:     make(http.Header),
		status:     http.StatusOK,
		size:       -1,
		onReady:    onReady,
		decided:    make(chan struct{}),
	}
}

func (w *channelRaceResponseWriter) Decide(winner bool) {
	w.decideOnce.Do(func() {
		w.winner.Store(winner)
		close(w.decided)
	})
}

func (w *channelRaceResponseWriter) Header() http.Header {
	return w.header
}

func (w *channelRaceResponseWriter) WriteHeader(code int) {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	if w.size >= 0 || code <= 0 {
		return
	}
	w.status = code
}

func (w *channelRaceResponseWriter) Write(data []byte) (int, error) {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	if len(data) == 0 {
		return 0, nil
	}
	if err := w.awaitDecision(); err != nil {
		return 0, err
	}
	w.commitHeaders()
	written, err := w.downstream.Write(data)
	w.size += written
	return written, err
}

func (w *channelRaceResponseWriter) WriteString(value string) (int, error) {
	return w.Write([]byte(value))
}

func (w *channelRaceResponseWriter) Status() int {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return w.status
}

func (w *channelRaceResponseWriter) Size() int {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return w.size
}

func (w *channelRaceResponseWriter) Written() bool {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return w.size >= 0
}

func (w *channelRaceResponseWriter) WriteHeaderNow() {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	if w.size < 0 {
		w.size = 0
	}
}

func (w *channelRaceResponseWriter) Flush() {
	select {
	case <-w.decided:
		if w.winner.Load() {
			w.downstream.Flush()
		}
	default:
	}
}

func (w *channelRaceResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errors.New("渠道组竞速不支持连接劫持")
}

func (w *channelRaceResponseWriter) CloseNotify() <-chan bool {
	return w.downstream.CloseNotify()
}

func (w *channelRaceResponseWriter) Pusher() http.Pusher {
	return w.downstream.Pusher()
}

func (w *channelRaceResponseWriter) awaitDecision() error {
	w.readyOnce.Do(w.onReady)
	select {
	case <-w.decided:
		if !w.winner.Load() {
			return errChannelRaceLost
		}
		return nil
	case <-w.ctx.Done():
		return w.ctx.Err()
	}
}

func (w *channelRaceResponseWriter) commitHeaders() {
	if w.committed {
		return
	}
	for key, values := range w.header {
		w.downstream.Header()[key] = append([]string(nil), values...)
	}
	w.downstream.WriteHeader(w.status)
	if w.size < 0 {
		w.size = 0
	}
	w.committed = true
}

var _ gin.ResponseWriter = (*channelRaceResponseWriter)(nil)
var _ io.StringWriter = (*channelRaceResponseWriter)(nil)
