package chaos

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// codedError tags an error with its stable event code and phase without
// changing its message.
type codedError struct {
	code  ErrorCode
	phase Phase
	err   error
}

func (e *codedError) Error() string { return e.err.Error() }
func (e *codedError) Unwrap() error { return e.err }

// NewUsageError marks err as a usage error (bad flags or arguments). The
// message is unchanged; run.end reports usage_error / invalid_argument.
func NewUsageError(err error) error {
	if err == nil {
		return nil
	}
	return &codedError{code: CodeInvalidArgument, phase: PhaseUsage, err: err}
}

// NewRuntimeUnavailableError marks err as a failure to reach the container
// runtime. The message is unchanged; run.end reports runtime_unavailable.
func NewRuntimeUnavailableError(err error) error {
	if err == nil {
		return nil
	}
	return &codedError{code: CodeRuntimeUnavailable, phase: PhaseRuntime, err: err}
}

// Outcome is the final result of a run: the run.end status and error, and the
// process exit code.
type Outcome struct {
	Status   RunStatus
	Error    *EventError
	ExitCode int
}

// NewOutcome maps the counted target results, the error the command returned
// and whether the run was interrupted (SIGINT/SIGTERM) to the run outcome.
// The exit code keeps Pumba's 0/1 vocabulary: 1 exactly when err is non-nil,
// so text-mode exit codes are unchanged. The status carries the detail.
func NewOutcome(counts Counts, err error, aborted bool) Outcome {
	exit := 0
	if err != nil {
		exit = 1
	}
	status, evErr := outcomeStatus(counts, err, aborted)
	return Outcome{Status: status, Error: evErr, ExitCode: exit}
}

// outcomeStatus is one ordered decision table: earlier rows take precedence.
func outcomeStatus(counts Counts, err error, aborted bool) (RunStatus, *EventError) {
	var coded *codedError
	isCoded := errors.As(err, &coded)
	switch {
	case isCoded && coded.code == CodeInvalidArgument, errors.Is(err, ErrNegativeMaxTargets):
		return RunUsageError, newEventError(CodeInvalidArgument, PhaseUsage, err)
	case errors.Is(err, ErrMaxTargetsExceeded):
		return RunRefused, newEventError(CodeMaxTargetsExceeded, PhaseSelect, err)
	case counts.CleanupFailed > 0:
		return RunCleanupFailed, &EventError{Code: CodeCleanupFailed, Phase: PhaseCleanup, Message: messageOr(err, fmt.Sprintf("cleanup failed on %d target(s)", counts.CleanupFailed))}
	case errors.Is(err, ErrMaxDurationExceeded):
		return RunAborted, newEventError(CodeTimeout, PhaseRuntime, err)
	case err != nil && isCoded:
		return failedOrPartial(counts), newEventError(coded.code, coded.phase, err)
	case err != nil:
		return failedOrPartial(counts), newEventError(CodeCommandFailed, PhaseInject, err)
	case counts.Failed > 0:
		return failedOrPartial(counts), &EventError{Code: CodeCommandFailed, Phase: PhaseInject, Message: fmt.Sprintf("%d target(s) failed", counts.Failed)}
	case aborted:
		return RunAborted, &EventError{Code: CodeCanceled, Phase: PhaseRuntime, Message: "run canceled"}
	case counts.Selected == 0:
		return RunNoTargets, &EventError{Code: CodeNoTargets, Phase: PhaseSelect, Message: "no containers found"}
	default:
		return RunOK, nil
	}
}

func failedOrPartial(counts Counts) RunStatus {
	if counts.OK+counts.Planned > 0 {
		return RunPartial
	}
	return RunFailed
}

func newEventError(code ErrorCode, phase Phase, err error) *EventError {
	return &EventError{Code: code, Phase: phase, Message: err.Error()}
}

func messageOr(err error, fallback string) string {
	if err != nil {
		return err.Error()
	}
	return fallback
}

// Recorder is the run-level sink cmd hands to every chaos builder. It counts
// target results synchronously, before delivery, so a slow or lossy delivery
// sink can never change the outcome; it remembers the run identity from
// run.start and stamps the Pumba version on it; then it forwards every event.
type Recorder struct {
	next    EventSink
	version string

	mu      sync.Mutex
	runID   string
	action  string
	counts  Counts
	skipped error
}

// skippedErrorRecorder is implemented by event sinks that keep the tick errors
// --skip-error swallowed, so run.end still reports them.
type skippedErrorRecorder interface {
	RecordSkipped(err error)
}

// RecordSkipped keeps the latest tick error --skip-error swallowed. Errors
// raised before any target result (listing, --target resolution) would
// otherwise leave run.end reporting no_targets.
func (r *Recorder) RecordSkipped(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skipped = err
}

// NewRecorder returns a Recorder forwarding to next.
func NewRecorder(next EventSink, version string) *Recorder {
	return &Recorder{next: next, version: version}
}

// Emit records ev and forwards it.
func (r *Recorder) Emit(ev *Event) {
	r.mu.Lock()
	if ev.Type == EventRunStart {
		r.runID, r.action = ev.RunID, ev.Action
		if ev.Start != nil {
			ev.Start.PumbaVersion = r.version
		}
	}
	if ev.Type == EventTargetResult && ev.Result != nil {
		r.count(ev.Result.Status)
	}
	r.mu.Unlock()
	r.next.Emit(ev)
}

func (r *Recorder) count(s TargetStatus) {
	r.counts.Selected++
	switch s {
	case TargetOK:
		r.counts.OK++
	case TargetFailed:
		r.counts.Failed++
	case TargetCleanupFailed:
		r.counts.CleanupFailed++
	case TargetPlanned:
		r.counts.Planned++
	case TargetSkipped:
		r.counts.Skipped++
	}
}

// RunEnd computes the run outcome from the recorded counts and returns it with
// the matching run.end event. The event carries the run ID and action seen on
// run.start (empty when the run failed before run.start); the caller sets
// EventsDropped and emits it to its delivery sink.
func (r *Recorder) RunEnd(err error, aborted bool) (Event, Outcome) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o := NewOutcome(r.counts, err, aborted)
	if err == nil && r.skipped != nil {
		// --skip-error keeps exit 0; the status still reports the failure
		o.Status, o.Error = outcomeStatus(r.counts, r.skipped, aborted)
	}
	return Event{
		Schema: EventSchema,
		Type:   EventRunEnd,
		RunID:  r.runID,
		Time:   time.Now().UTC(),
		Action: r.action,
		End:    &RunEnd{Status: o.Status, Counts: r.counts, Error: o.Error},
	}, o
}
