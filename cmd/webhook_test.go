package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

const webhookToken = "secret-token"

// syncBuffer is a bytes.Buffer safe for the webhook worker and the test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// receiver records every POST and answers with status.
type receiver struct {
	*httptest.Server
	mu           sync.Mutex
	bodies       []string
	contentTypes []string
}

func newReceiver(t *testing.T, status int) *receiver {
	t.Helper()
	r := &receiver{}
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		assert.NoError(t, err)
		assert.Equal(t, http.MethodPost, req.Method)
		r.mu.Lock()
		r.bodies = append(r.bodies, string(body))
		r.contentTypes = append(r.contentTypes, req.Header.Get("Content-Type"))
		r.mu.Unlock()
		http.Error(w, "secret response body", status)
	}))
	t.Cleanup(r.Close)
	return r
}

func (r *receiver) received() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.bodies...)
}

// newStalledServer accepts requests and holds them until the client gives up
// or the test ends. started receives one value per request.
func newStalledServer(t *testing.T) (*httptest.Server, chan struct{}) {
	t.Helper()
	release := make(chan struct{})
	started := make(chan struct{}, eventQueueSize)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		started <- struct{}{}
		_, _ = io.Copy(io.Discard, req.Body) // lets the server notice the client giving up
		select {
		case <-release:
		case <-req.Context().Done():
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) }) // runs first: Cleanup is LIFO
	return server, started
}

func assertOneWarningWithoutSecrets(t *testing.T, stderr, url string) {
	t.Helper()
	assert.Equal(t, 1, strings.Count(stderr, "warning:"), "stderr: %q", stderr)
	assert.NotContains(t, stderr, url)
	assert.NotContains(t, stderr, webhookToken)
	assert.NotContains(t, stderr, "secret response body")
	assert.NotContains(t, stderr, `"schema"`)
}

func assertAsyncWarningWithoutSecrets(t *testing.T, stderr *syncBuffer, url string) {
	t.Helper()
	require.Eventually(t, func() bool { return strings.Count(stderr.String(), "warning:") == 1 }, time.Second, time.Millisecond)
	assertOneWarningWithoutSecrets(t, stderr.String(), url)
}

func TestWebhookPostsNDJSONEventsInOrder(t *testing.T) {
	r := newReceiver(t, http.StatusNoContent)
	var stderr syncBuffer
	hook := newWebhookSink(r.URL+"/"+webhookToken, time.Second, &stderr, eventQueueSize)
	var want bytes.Buffer
	stream := newStreamSink(&want, eventQueueSize)
	const n = 20
	for i := range int64(n) {
		hook.Emit(appliedEvent(i + 1))
		stream.Emit(appliedEvent(i + 1))
	}
	require.True(t, hook.flush(runEndEvent(), time.Second))
	require.True(t, stream.flush(runEndEvent(), time.Second))

	lines := strings.SplitAfter(want.String(), "\n")
	assert.Equal(t, lines[:len(lines)-1], r.received(), "one ndjson line per request, in order")
	for _, ct := range r.contentTypes {
		assert.Equal(t, "application/json", ct)
	}
	assert.Zero(t, hook.dropped.Load())
	assert.Empty(t, stderr.String())
}

func TestWebhookDeliveryFailures(t *testing.T) {
	tests := []struct {
		name    string
		handler func(w http.ResponseWriter, r *http.Request, redirectTo string)
	}{
		{name: "server error", handler: func(w http.ResponseWriter, _ *http.Request, _ string) {
			http.Error(w, "secret response body", http.StatusInternalServerError)
		}},
		{name: "client error", handler: func(w http.ResponseWriter, _ *http.Request, _ string) {
			http.Error(w, "secret response body", http.StatusUnauthorized)
		}},
		{name: "redirect", handler: func(w http.ResponseWriter, r *http.Request, to string) {
			http.Redirect(w, r, to, http.StatusTemporaryRedirect)
		}},
		{name: "closed connection", handler: func(w http.ResponseWriter, _ *http.Request, _ string) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
		}},
		{name: "timeout", handler: func(_ http.ResponseWriter, r *http.Request, _ string) {
			_, _ = io.Copy(io.Discard, r.Body) // lets the server notice the client giving up
			<-r.Context().Done()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redirect := newReceiver(t, http.StatusOK)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.handler(w, r, redirect.URL)
			}))
			t.Cleanup(server.Close)
			url := server.URL + "/" + webhookToken
			var stderr syncBuffer
			hook := newWebhookSink(url, 50*time.Millisecond, &stderr, eventQueueSize)

			hook.Emit(appliedEvent(1))
			hook.Emit(appliedEvent(2))
			require.True(t, hook.flush(runEndEvent(), time.Second))

			assert.Equal(t, int64(3), hook.dropped.Load(), "every failed request counts")
			assert.Empty(t, redirect.received(), "redirects are not followed")
			assertAsyncWarningWithoutSecrets(t, &stderr, url)
		})
	}
}

func TestWebhookStalledReceiverNeverBlocksEmit(t *testing.T) {
	server, started := newStalledServer(t)
	var stderr syncBuffer
	const queue, extra = 4, 5
	hook := newWebhookSink(server.URL+"/"+webhookToken, time.Minute, &stderr, queue)
	hook.Emit(appliedEvent(1))
	<-started // the worker holds event 1 and is stuck

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range int64(queue + extra) {
			hook.Emit(appliedEvent(i + 2))
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Emit blocked on a stalled webhook")
	}

	assert.Equal(t, int64(extra), hook.dropped.Load(), "events beyond the queue are dropped and counted")
	assert.False(t, hook.flush(runEndEvent(), 20*time.Millisecond), "flush is bounded")
	assertAsyncWarningWithoutSecrets(t, &stderr, server.URL)
}

func TestFinishWebhookFlushIsBoundedAndOutcomeIndependent(t *testing.T) {
	tests := []struct {
		name     string
		status   chaos.TargetStatus
		err      error
		exitCode int
	}{
		{name: "ok", status: chaos.TargetOK},
		{name: "failed", status: chaos.TargetFailed, err: assert.AnError, exitCode: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := newStalledServer(t)
			var stderr syncBuffer
			o := newEventOutput(io.Discard, &stderr, "v")
			o.hookTimeout = 50 * time.Millisecond
			require.NoError(t, o.enableWebhook(server.URL))
			gp := &chaos.GlobalParams{RunID: "r1", Action: "kill", Events: o.rec}
			gp.Emit(&chaos.Event{Type: chaos.EventRunStart, Start: &chaos.RunStart{}})
			for range eventQueueSize + 10 {
				gp.Emit(&chaos.Event{Type: chaos.EventTargetResult, Result: &chaos.TargetResult{Status: tt.status}})
			}

			start := time.Now()
			assert.Equal(t, tt.exitCode, o.finish(tt.err, false))
			assert.Less(t, time.Since(start), time.Second)
			assertAsyncWarningWithoutSecrets(t, &stderr, server.URL)
		})
	}
}

func TestValidateWebhookURL(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{url: "http://collector:8080/events", valid: true},
		{url: "https://hooks.example.com/" + webhookToken + "?k=v", valid: true},
		{url: "HTTPS://hooks.example.com", valid: true},
		{url: "ftp://hooks.example.com/" + webhookToken},
		{url: "file:///etc/" + webhookToken},
		{url: "hooks.example.com/" + webhookToken},
		{url: "localhost:8080"},
		{url: "http://"},
		{url: "http://:80/events"},
		{url: "https://user@:443/events"},
		{url: "http:///" + webhookToken},
		{url: "://" + webhookToken},
		{url: "http://bad host/" + webhookToken},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			err := validateWebhookURL(tt.url)
			if tt.valid {
				assert.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, errWebhookURL)
			assert.NotContains(t, err.Error(), webhookToken)
		})
	}
}

func TestRunWebhookReceivesEventsInEveryOutputMode(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		killErr  error
		status   chaos.RunStatus
		exitCode int
	}{
		{name: "text ok", output: outputText, status: chaos.RunOK},
		{name: "text failed", output: outputText, killErr: assert.AnError, status: chaos.RunFailed, exitCode: 1},
		{name: "ndjson ok", output: outputNDJSON, status: chaos.RunOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReceiver(t, http.StatusOK)
			code, stdout, stderr := runPumba(t, mockKill(t, ctr.CreateTestContainers(1), tt.killErr),
				"-o", tt.output, "--run-id", "r1", "--webhook", r.URL, "kill", "c0")

			assert.Equal(t, tt.exitCode, code)
			got := parseNDJSON(t, strings.Join(r.received(), ""))
			require.NotEmpty(t, got)
			assert.Equal(t, chaos.EventRunStart, got[0].Type)
			end := got[len(got)-1]
			assert.Equal(t, chaos.EventRunEnd, end.Type)
			assert.Equal(t, string(tt.status), end.Status)
			assert.Equal(t, "r1", end.RunID)
			if tt.output == outputText {
				assert.Empty(t, stdout, "text mode stdout is unchanged")
			} else {
				assert.Equal(t, types(parseNDJSON(t, stdout)), types(got), "webhook mirrors stdout")
			}
			assert.NotContains(t, stderr, "warning:")
		})
	}
}

func TestRunWebhookFailureKeepsOutcome(t *testing.T) {
	r := newReceiver(t, http.StatusInternalServerError)
	url := r.URL + "/" + webhookToken
	code, stdout, stderr := runPumba(t, mockKill(t, ctr.CreateTestContainers(1), nil),
		"-o", "ndjson", "--webhook", url, "kill", "c0")

	assert.Zero(t, code)
	lines := parseNDJSON(t, stdout)
	end := lines[len(lines)-1]
	assert.Equal(t, string(chaos.RunOK), end.Status)
	require.NotNil(t, end.EventsDropped)
	assert.Zero(t, *end.EventsDropped, "stdout run.end reports stdout drops only")
	assert.Len(t, r.received(), len(lines), "every event was attempted")
	assertOneWarningWithoutSecrets(t, stderr, url)
	assert.NotContains(t, stdout, webhookToken)
}

func TestWebhookUsageErrorDoesNotExitBeforeFlush(t *testing.T) {
	for _, output := range []string{outputText, outputNDJSON} {
		t.Run(output, func(t *testing.T) {
			var exits []int
			exiter := cli.OsExiter
			cli.OsExiter = func(code int) { exits = append(exits, code) }
			t.Cleanup(func() { cli.OsExiter = exiter })
			r := newReceiver(t, http.StatusNoContent)
			client := ctr.NewMockClient(t)
			client.EXPECT().Close().Return(nil).Maybe()
			code, _, _ := runPumba(t, client, "-o", output, "--webhook", r.URL, "bogus")

			require.Empty(t, exits, "urfave must return to finish instead of immediately exiting")
			require.Equal(t, 1, code)
			bodies := r.received()
			require.Len(t, bodies, 1)
			lines := parseNDJSON(t, bodies[0])
			require.Equal(t, []chaos.EventType{chaos.EventRunEnd}, types(lines))
			require.Equal(t, string(chaos.RunUsageError), lines[0].Status)
		})
	}
}

func TestRunInvalidWebhookIsUsageError(t *testing.T) {
	for _, output := range []string{outputText, outputNDJSON} {
		t.Run(output, func(t *testing.T) {
			client := ctr.NewMockClient(t)
			client.EXPECT().Close().Return(nil).Maybe()
			url := "ftp://hooks.example.com/" + webhookToken
			code, stdout, stderr := runPumba(t, client, "-o", output, "--webhook", url, "kill", "c0")

			assert.Equal(t, 1, code)
			assert.NotContains(t, stdout+stderr, webhookToken)
			if output == outputText {
				assert.Empty(t, stdout)
				return
			}
			lines := parseNDJSON(t, stdout)
			require.Equal(t, []chaos.EventType{chaos.EventRunEnd}, types(lines))
			assert.Equal(t, string(chaos.RunUsageError), lines[0].Status)
			require.NotNil(t, lines[0].Error)
			assert.Equal(t, chaos.CodeInvalidArgument, lines[0].Error.Code)
		})
	}
}
