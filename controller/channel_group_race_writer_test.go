package controller

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelRaceResponseWriter_commitsOnlyWinner(t *testing.T) {
	recorder := httptest.NewRecorder()
	base, _ := gin.CreateTestContext(recorder)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ready := make(chan struct{}, 1)
	writer := newChannelRaceResponseWriter(ctx, base.Writer, func() { ready <- struct{}{} })
	result := make(chan error, 1)

	go func() {
		_, err := writer.Write([]byte("data: winner\n\n"))
		result <- err
	}()
	<-ready
	writer.Decide(true)

	require.NoError(t, <-result)
	require.Equal(t, "data: winner\n\n", recorder.Body.String())
}

func TestChannelRaceResponseWriter_discardsLoser(t *testing.T) {
	recorder := httptest.NewRecorder()
	base, _ := gin.CreateTestContext(recorder)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ready := make(chan struct{}, 1)
	writer := newChannelRaceResponseWriter(ctx, base.Writer, func() { ready <- struct{}{} })
	result := make(chan error, 1)

	go func() {
		_, err := writer.Write([]byte("data: loser\n\n"))
		result <- err
	}()
	<-ready
	writer.Decide(false)

	require.ErrorIs(t, <-result, errChannelRaceLost)
	require.Empty(t, recorder.Body.String())
}
