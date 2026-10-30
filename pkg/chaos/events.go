package chaos

import (
	"encoding/json"
	"time"
)

// EventSchema versions the result event stream. Within v1, fields and event
// types may be added; renaming, removing or re-meaning a field, or adding a
// status or error code value, bumps the version.
const EventSchema = "pumba.event/v1"

// EventType names one kind of result event.
type EventType string

// Event types of pumba.event/v1.
const (
	EventRunStart      EventType = "run.start"
	EventTargetApplied EventType = "target.applied"
	EventTargetResult  EventType = "target.result"
	EventRunEnd        EventType = "run.end"
)

// TargetStatus is the terminal status of one target in one tick.
type TargetStatus string

// Target statuses reported in target.result.
const (
	TargetOK            TargetStatus = "ok"
	TargetFailed        TargetStatus = "failed"
	TargetCleanupFailed TargetStatus = "cleanup_failed"
	TargetPlanned       TargetStatus = "planned"
	TargetSkipped       TargetStatus = "skipped"
)

// RunStatus is the final status of a run reported in run.end.
type RunStatus string

// Run statuses reported in run.end.
const (
	RunOK            RunStatus = "ok"
	RunFailed        RunStatus = "failed"
	RunPartial       RunStatus = "partial"
	RunCleanupFailed RunStatus = "cleanup_failed"
	RunNoTargets     RunStatus = "no_targets"
	RunRefused       RunStatus = "refused"
	RunUsageError    RunStatus = "usage_error"
	RunAborted       RunStatus = "aborted"
)

// ErrorCode is the stable, machine-readable error classification. Agents
// branch on it; the message is for humans.
type ErrorCode string

// Error codes of pumba.event/v1.
const (
	CodeInvalidArgument     ErrorCode = "invalid_argument"
	CodeNoTargets           ErrorCode = "no_targets"
	CodeMaxTargetsExceeded  ErrorCode = "max_targets_exceeded"
	CodeRuntimeUnavailable  ErrorCode = "runtime_unavailable"
	CodeRootlessUnsupported ErrorCode = "rootless_unsupported"
	CodeCommandFailed       ErrorCode = "command_failed"
	CodeCleanupFailed       ErrorCode = "cleanup_failed"
	CodeTimeout             ErrorCode = "timeout"
	CodeCanceled            ErrorCode = "canceled"
)

// Phase names the stage of a run an error belongs to.
type Phase string

// Error phases of pumba.event/v1.
const (
	PhaseUsage   Phase = "usage"
	PhaseSelect  Phase = "select"
	PhaseInject  Phase = "inject"
	PhaseCleanup Phase = "cleanup"
	PhaseInspect Phase = "inspect"
	PhaseRuntime Phase = "runtime"
)

// Event is one pumba.event/v1 record. The header fields are common to every
// type; exactly one body pointer matching Type is set.
type Event struct {
	Schema string    `json:"schema"`
	Type   EventType `json:"type"`
	RunID  string    `json:"run_id"`
	Time   time.Time `json:"time"`
	Action string    `json:"action"`

	Start   *RunStart      `json:"-"`
	Applied *TargetApplied `json:"-"`
	Result  *TargetResult  `json:"-"`
	End     *RunEnd        `json:"-"`
}

// RunStart is the body of run.start: flags parsed, command built.
type RunStart struct {
	PumbaVersion string         `json:"pumba_version"`
	Runtime      string         `json:"runtime"`
	DryRun       bool           `json:"dry_run"`
	Interval     string         `json:"interval"`
	Selection    Selection      `json:"selection"`
	Params       map[string]any `json:"params"`
}

// Selection describes how targets are chosen.
type Selection struct {
	Names      []string `json:"names"`
	Pattern    string   `json:"pattern,omitempty"`
	Labels     []string `json:"labels"`
	Random     bool     `json:"random"`
	MaxTargets int      `json:"max_targets"`
}

// TargetRef identifies a target container.
type TargetRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TargetApplied is the body of target.applied: the fault is active on the
// target (for one-shot actions: the action ran).
type TargetApplied struct {
	Tick   int64     `json:"tick"`
	Target TargetRef `json:"target"`
}

// TargetResult is the body of target.result, the terminal record per target
// and tick. It is emitted only after the fault was reverted.
type TargetResult struct {
	Tick    int64        `json:"tick"`
	Target  TargetRef    `json:"target"`
	Started time.Time    `json:"started"`
	Ended   time.Time    `json:"ended"`
	Status  TargetStatus `json:"status"`
	Error   *EventError  `json:"error"`
	// Cleanup is nil when the action leaves nothing to revert.
	Cleanup *Cleanup `json:"cleanup"`
}

// Cleanup is the outcome of reverting a fault on one target.
type Cleanup struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// RunEnd is the body of run.end.
type RunEnd struct {
	Status        RunStatus   `json:"status"`
	Counts        Counts      `json:"counts"`
	Error         *EventError `json:"error"`
	EventsDropped int64       `json:"events_dropped"`
}

// Counts tallies target.result statuses over the whole run.
type Counts struct {
	Selected      int `json:"selected"`
	OK            int `json:"ok"`
	Failed        int `json:"failed"`
	CleanupFailed int `json:"cleanup_failed"`
	Planned       int `json:"planned"`
	Skipped       int `json:"skipped"`
}

// EventError is the error object of target.result and run.end.
type EventError struct {
	Code    ErrorCode `json:"code"`
	Phase   Phase     `json:"phase"`
	Message string    `json:"message"`
}

// MarshalJSON flattens the header and the body of the event's type into one
// JSON object, header first.
//
//nolint:gocritic // value receiver, so json.Marshal handles Event values and pointers alike
func (e Event) MarshalJSON() ([]byte, error) {
	type header Event // drops this method, avoiding recursion
	h := header(e)
	switch e.Type {
	case EventRunStart:
		return json.Marshal(struct {
			header
			*RunStart
		}{h, e.Start})
	case EventTargetApplied:
		return json.Marshal(struct {
			header
			*TargetApplied
		}{h, e.Applied})
	case EventTargetResult:
		return json.Marshal(struct {
			header
			*TargetResult
		}{h, e.Result})
	case EventRunEnd:
		return json.Marshal(struct {
			header
			*RunEnd
		}{h, e.End})
	default:
		return json.Marshal(h)
	}
}

// EventSink receives result events. Implementations must be safe for
// concurrent use and must not block: chaos emits while containers are paused
// or faults are active. Emit takes ownership of ev; the caller does not touch
// it afterwards.
type EventSink interface {
	Emit(ev *Event)
}

// NopSink discards every event.
type NopSink struct{}

// Emit discards ev.
func (NopSink) Emit(*Event) {}

// Emit stamps the event header from gp (schema, run ID, action, time) and
// hands the event to gp.Events. A GlobalParams without a sink discards events,
// so direct callers and tests need not set one.
func (gp *GlobalParams) Emit(ev *Event) {
	if gp.Events == nil {
		return
	}
	ev.Schema = EventSchema
	ev.RunID = gp.RunID
	ev.Action = gp.Action
	if ev.Time.IsZero() {
		ev.Time = time.Now().UTC()
	}
	gp.Events.Emit(ev)
}
