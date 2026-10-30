package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

// eventLine is the subset of pumba.event/v1 fields these tests assert on.
type eventLine struct {
	Type         chaos.EventType `json:"type"`
	RunID        string          `json:"run_id"`
	Action       string          `json:"action"`
	PumbaVersion string          `json:"pumba_version"`
	Tick         int64           `json:"tick"`
	Status       string          `json:"status"`
	Error        *struct {
		Code  chaos.ErrorCode `json:"code"`
		Phase chaos.Phase     `json:"phase"`
	} `json:"error"`
	EventsDropped *int64 `json:"events_dropped"`
}

// parseNDJSON fails the test unless every stdout line is one JSON object.
func parseNDJSON(t *testing.T, stdout string) []eventLine {
	t.Helper()
	var lines []eventLine
	for raw := range strings.SplitSeq(strings.TrimSuffix(stdout, "\n"), "\n") {
		if raw == "" && stdout == "" {
			break
		}
		require.True(t, json.Valid([]byte(raw)), "stdout line is not JSON: %q", raw)
		var l eventLine
		require.NoError(t, json.Unmarshal([]byte(raw), &l))
		lines = append(lines, l)
	}
	return lines
}

func types(lines []eventLine) []chaos.EventType {
	out := make([]chaos.EventType, 0, len(lines))
	for _, l := range lines {
		out = append(out, l.Type)
	}
	return out
}

// runPumba runs the CLI against client (via a stubbed Docker factory) and
// returns the exit code, stdout and the CLI's help/usage writer output.
func runPumba(t *testing.T, client ctr.Client, args ...string) (int, string, string) {
	t.Helper()
	restoreFactories(t)
	newDockerClient = func(string, *tls.Config) (ctr.Client, error) { return client, nil }
	t.Cleanup(func() { runtimeClient = nil })
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"pumba"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func mockKill(t *testing.T, targets []*ctr.Container, killErr error) *ctr.MockClient {
	t.Helper()
	client := ctr.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), ctr.ListOpts{}).Return(targets, nil).Maybe()
	client.EXPECT().KillContainer(mock.Anything, mock.AnythingOfType("*container.Container"), "SIGKILL", false).Return(killErr).Maybe()
	client.EXPECT().Close().Return(nil).Maybe()
	return client
}

func TestRunNDJSONOutcomes(t *testing.T) {
	tests := []struct {
		name     string
		targets  []*ctr.Container
		killErr  error
		types    []chaos.EventType
		status   chaos.RunStatus
		code     chaos.ErrorCode
		exitCode int
	}{
		{
			name: "ok", targets: ctr.CreateTestContainers(1),
			types:  []chaos.EventType{chaos.EventRunStart, chaos.EventTargetApplied, chaos.EventTargetResult, chaos.EventRunEnd},
			status: chaos.RunOK,
		},
		{
			name: "failed", targets: ctr.CreateTestContainers(1), killErr: errors.New("boom"),
			types:  []chaos.EventType{chaos.EventRunStart, chaos.EventTargetResult, chaos.EventRunEnd},
			status: chaos.RunFailed, code: chaos.CodeCommandFailed, exitCode: 1,
		},
		{
			name:   "no targets",
			types:  []chaos.EventType{chaos.EventRunStart, chaos.EventRunEnd},
			status: chaos.RunNoTargets, code: chaos.CodeNoTargets,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, _ := runPumba(t, mockKill(t, tt.targets, tt.killErr), "-o", "ndjson", "--run-id", "r1", "kill", "c0")

			assert.Equal(t, tt.exitCode, code)
			lines := parseNDJSON(t, stdout)
			require.Equal(t, tt.types, types(lines))
			for _, l := range lines {
				assert.Equal(t, "r1", l.RunID)
				assert.Equal(t, "kill", l.Action)
			}
			assert.Equal(t, version, lines[0].PumbaVersion)
			end := lines[len(lines)-1]
			assert.Equal(t, string(tt.status), end.Status)
			require.NotNil(t, end.EventsDropped)
			assert.Zero(t, *end.EventsDropped)
			if tt.code == "" {
				assert.Nil(t, end.Error)
				return
			}
			require.NotNil(t, end.Error)
			assert.Equal(t, tt.code, end.Error.Code)
		})
	}
}

func TestRunTextModeKeepsStdoutEmpty(t *testing.T) {
	code, stdout, _ := runPumba(t, mockKill(t, ctr.CreateTestContainers(1), errors.New("boom")), "kill", "c0")
	assert.Equal(t, 1, code)
	assert.Empty(t, stdout)
}

func TestRunGeneratesOneRunID(t *testing.T) {
	code, stdout, _ := runPumba(t, mockKill(t, ctr.CreateTestContainers(1), nil), "-o", "ndjson", "kill", "c0")
	require.Zero(t, code)
	lines := parseNDJSON(t, stdout)
	require.NotEmpty(t, lines)
	assert.Regexp(t, `^[0-9a-f]{12}$`, lines[0].RunID)
	for _, l := range lines {
		assert.Equal(t, lines[0].RunID, l.RunID)
	}
}

func TestConfigureWritesRunIDBackForGlobalParams(t *testing.T) {
	for _, given := range []string{"", "ci-42"} {
		t.Run("run-id="+given, func(t *testing.T) {
			c := newRuntimeTestContext(t, map[string]string{"run-id": given})
			o := newEventOutput(io.Discard, io.Discard, "v")
			require.NoError(t, o.configure(c))
			gp, err := chaos.ParseGlobalParams(cliflags.V1{Ctx: c})
			require.NoError(t, err)
			assert.Equal(t, o.runID, gp.RunID)
			if given != "" {
				assert.Equal(t, given, gp.RunID)
			}
		})
	}
}

func TestRunUsageErrorsEndWithRunEnd(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		runID     string
		usageText bool
	}{
		{name: "invalid run id", args: []string{"-o", "ndjson", "--run-id", "bad id", "kill", "c0"}},
		{name: "unknown global flag", args: []string{"-o", "ndjson", "--run-id", "r1", "--bogus", "kill", "c0"}, runID: "r1", usageText: true},
		{name: "bad command flag value", args: []string{"-o", "ndjson", "--run-id", "r1", "kill", "--limit", "abc", "c0"}, runID: "r1", usageText: true},
		{name: "bad parent flag value", args: []string{"-o", "ndjson", "--run-id", "r1", "netem", "--duration", "abc", "delay", "c0"}, runID: "r1", usageText: true},
		{name: "missing container", args: []string{"-o", "ndjson", "--run-id", "r1", "kill"}, runID: "r1"},
		{name: "negative max targets", args: []string{"-o", "ndjson", "--run-id", "r1", "--max-targets", "-1", "kill", "c0"}, runID: "r1"},
		{name: "negative max duration", args: []string{"-o", "ndjson", "--run-id", "r1", "--max-duration", "-1s", "kill", "c0"}, runID: "r1"},
		{name: "unknown command", args: []string{"-o", "ndjson", "--run-id", "r1", "bogus"}, runID: "r1"},
		{name: "unsupported runtime", args: []string{"-o", "ndjson", "--run-id", "r1", "--runtime", "rkt", "kill", "c0"}, runID: "r1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := ctr.NewMockClient(t)
			client.EXPECT().Close().Return(nil).Maybe()

			code, stdout, stderr := runPumba(t, client, tt.args...)

			assert.Equal(t, 1, code)
			lines := parseNDJSON(t, stdout)
			require.Equal(t, []chaos.EventType{chaos.EventRunEnd}, types(lines))
			end := lines[0]
			assert.Equal(t, string(chaos.RunUsageError), end.Status)
			require.NotNil(t, end.Error)
			assert.Equal(t, chaos.CodeInvalidArgument, end.Error.Code)
			assert.Equal(t, chaos.PhaseUsage, end.Error.Phase)
			if tt.runID != "" {
				assert.Equal(t, tt.runID, end.RunID)
			}
			if tt.usageText {
				assert.Contains(t, stderr, "Incorrect Usage")
			}
		})
	}
}

func TestRunRuntimeUnavailable(t *testing.T) {
	restoreFactories(t)
	newDockerClient = func(string, *tls.Config) (ctr.Client, error) { return nil, errors.New("dial") }
	var stdout bytes.Buffer
	code := run([]string{"pumba", "-o", "ndjson", "kill", "c0"}, &stdout, io.Discard)

	assert.Equal(t, 1, code)
	lines := parseNDJSON(t, stdout.String())
	require.Len(t, lines, 1)
	require.NotNil(t, lines[0].Error)
	assert.Equal(t, chaos.CodeRuntimeUnavailable, lines[0].Error.Code)
}

func TestRunHelpIsNotARun(t *testing.T) {
	client := ctr.NewMockClient(t)
	client.EXPECT().Close().Return(nil)
	code, stdout, stderr := runPumba(t, client, "-o", "ndjson", "netem")
	assert.Zero(t, code)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "USAGE")
}

func TestRunMaxDurationExpiry(t *testing.T) {
	start := time.Now()
	code, stdout, _ := runPumba(t, mockKill(t, ctr.CreateTestContainers(1), nil),
		"-o", "ndjson", "--max-duration", "100ms", "--interval", "20ms", "kill", "c0")

	assert.Equal(t, 1, code)
	assert.Less(t, time.Since(start), 5*time.Second)
	lines := parseNDJSON(t, stdout)
	end := lines[len(lines)-1]
	assert.Equal(t, chaos.EventRunEnd, end.Type)
	assert.Equal(t, string(chaos.RunAborted), end.Status)
	require.NotNil(t, end.Error)
	assert.Equal(t, chaos.CodeTimeout, end.Error.Code)
}

// TestUsageTextUnchanged pins the usage text printed by onUsageError to what
// urfave/cli prints without a handler, at every level that can fail parsing.
func TestUsageTextUnchanged(t *testing.T) {
	for _, args := range [][]string{
		{"pumba", "--bogus", "kill", "c0"},
		{"pumba", "kill", "--bogus", "c0"},
		{"pumba", "netem", "--bogus", "delay", "c0"},
		{"pumba", "netem", "delay", "--bogus", "c0"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			restoreFactories(t)
			client := ctr.NewMockClient(t)
			client.EXPECT().Close().Return(nil).Maybe()
			newDockerClient = func(string, *tls.Config) (ctr.Client, error) { return client, nil }
			t.Cleanup(func() { runtimeClient = nil })
			usage := func(handler bool) string {
				var stdout bytes.Buffer
				app := newApp(newEventOutput(&stdout, io.Discard, "v"))
				app.Compiled = time.Time{}
				if !handler {
					app.OnUsageError = nil
					setUsageErrorHandler(app.Commands, nil)
				}
				require.Error(t, app.Run(args))
				return stdout.String()
			}
			want := usage(false)
			assert.Contains(t, want, "Incorrect Usage")
			assert.Equal(t, want, usage(true))
		})
	}
}

// blockingWriter blocks every Write until released.
type blockingWriter struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingWriter(t *testing.T) *blockingWriter {
	w := &blockingWriter{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { close(w.release) })
	return w
}

func (w *blockingWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return len(p), nil
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func appliedEvent(tick int64) *chaos.Event {
	return &chaos.Event{Schema: chaos.EventSchema, Type: chaos.EventTargetApplied, RunID: "r1", Action: "kill", Applied: &chaos.TargetApplied{Tick: tick}}
}

func runEndEvent() *chaos.Event {
	return &chaos.Event{Schema: chaos.EventSchema, Type: chaos.EventRunEnd, RunID: "r1", Action: "kill", End: &chaos.RunEnd{Status: chaos.RunOK}}
}

func TestStreamSinkWritesOrderedLines(t *testing.T) {
	var buf bytes.Buffer
	s := newStreamSink(&buf, eventQueueSize)
	const n = 50
	for i := range int64(n) {
		s.Emit(appliedEvent(i + 1))
	}
	require.True(t, s.flush(runEndEvent(), time.Second))

	lines := parseNDJSON(t, buf.String())
	require.Len(t, lines, n+1)
	for i, l := range lines[:n] {
		assert.Equal(t, int64(i+1), l.Tick)
	}
	assert.Equal(t, chaos.EventRunEnd, lines[n].Type)
}

func TestStreamSinkStalledWriterDropsInsteadOfBlocking(t *testing.T) {
	w := newBlockingWriter(t)
	const queue, extra = 4, 5
	s := newStreamSink(w, queue)
	s.Emit(appliedEvent(1))
	<-w.entered // the writer holds event 1 and is stuck

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range int64(queue + extra) {
			s.Emit(appliedEvent(i + 2))
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Emit blocked on a stalled writer")
	}

	s.Emit(appliedEvent(99))
	assert.False(t, s.flush(runEndEvent(), 20*time.Millisecond), "flush is bounded")
	assert.Equal(t, int64(extra+1), s.dropped.Load(), "events after stop are dropped")
}

func TestStreamSinkCountsWriteFailures(t *testing.T) {
	s := newStreamSink(failingWriter{}, eventQueueSize)
	s.Emit(appliedEvent(1))
	end := runEndEvent()
	require.True(t, s.flush(end, time.Second))
	assert.Equal(t, int64(1), end.End.EventsDropped, "write failures while draining count; the run.end line itself does not")
	assert.Equal(t, int64(2), s.dropped.Load())
}

func TestFinishOutcomeIgnoresStalledStdout(t *testing.T) {
	tests := []struct {
		name     string
		status   chaos.TargetStatus
		err      error
		exitCode int
	}{
		{name: "ok", status: chaos.TargetOK},
		{name: "failed", status: chaos.TargetFailed, err: errors.New("boom"), exitCode: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := newEventOutput(newBlockingWriter(t), io.Discard, "v")
			o.flushTimeout = 20 * time.Millisecond
			o.enableNDJSON(cli.NewApp())
			gp := &chaos.GlobalParams{RunID: "r1", Action: "kill", Events: o.rec}
			gp.Emit(&chaos.Event{Type: chaos.EventRunStart, Start: &chaos.RunStart{}})
			for range eventQueueSize + 10 {
				gp.Emit(&chaos.Event{Type: chaos.EventTargetResult, Result: &chaos.TargetResult{Status: tt.status}})
			}

			start := time.Now()
			assert.Equal(t, tt.exitCode, o.finish(tt.err, false))
			assert.Less(t, time.Since(start), time.Second)
			_, outcome := o.rec.RunEnd(tt.err, false)
			assert.NotEqual(t, chaos.RunNoTargets, outcome.Status, "dropped events still count")
		})
	}
}
