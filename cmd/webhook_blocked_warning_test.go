package main

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/stretchr/testify/require"
)

func TestWebhookWarningCannotBlockProducer(t *testing.T) {
	var hook *streamSink
	t.Cleanup(func() {
		if hook != nil {
			hook.flush(nil, time.Second)
		}
	})
	stderr := newBlockingWriter(t)
	server, started := newStalledServer(t)
	hook = newWebhookSink(server.URL, time.Minute, stderr, 1)
	hook.Emit(appliedEvent(1))
	<-started
	hook.Emit(appliedEvent(2))
	done := make(chan struct{})
	go func() { hook.Emit(appliedEvent(3)); close(done) }()
	<-stderr.entered
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("queue-overflow warning blocked the event producer")
	}
}

func TestWebhookWarningCannotExtendFinishBound(t *testing.T) {
	finished := make(chan struct{})
	started := false
	t.Cleanup(func() {
		if started {
			<-finished
		}
	})
	stderr := newBlockingWriter(t)
	server := newReceiver(t, http.StatusInternalServerError)
	o := newEventOutput(io.Discard, stderr, "test")
	o.hookTimeout = 20 * time.Millisecond
	require.NoError(t, o.enableWebhook(server.URL))
	gp := &chaos.GlobalParams{RunID: "r1", Action: "kill", Events: o.rec}
	gp.Emit(&chaos.Event{Type: chaos.EventRunStart, Start: &chaos.RunStart{}})
	<-stderr.entered
	done := make(chan int, 1)
	started = true
	go func() { defer close(finished); done <- o.finish(nil, false) }()
	select {
	case code := <-done:
		require.Zero(t, code)
	case <-time.After(time.Second):
		t.Fatal("warning blocked bounded event shutdown")
	}
}
