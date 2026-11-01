package chaos

import (
	"errors"
	"sync"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
)

// Target is the per-target handle RunOnContainers passes to a ContainerAction.
// The action reports through it when the fault becomes active and how its
// revert went; RunOnContainers turns those reports into target.applied and
// target.result events. Safe for concurrent use.
type Target struct {
	gp      *GlobalParams
	tick    int64
	ref     TargetRef
	started time.Time

	mu         sync.Mutex
	applied    bool
	reverted   bool
	revertErr  error
	deferred   bool
	terminated bool
	state      *TargetState
}

func newTarget(gp *GlobalParams, tick int64, c *container.Container) *Target {
	return &Target{gp: gp, tick: tick, ref: TargetRef{ID: c.ID(), Name: c.Name()}, started: time.Now().UTC()}
}

// Applied reports that the fault is now active on the target: after inject,
// before the duration wait. One-shot actions call it once the action ran.
// It emits target.applied once; dry-runs apply nothing and emit nothing.
func (t *Target) Applied() {
	if t.gp.DryRun {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.applied || t.terminated {
		return
	}
	t.applied = true
	t.gp.Emit(&Event{Type: EventTargetApplied, Applied: &TargetApplied{Tick: t.tick, Target: t.ref}})
}

// Reverted records the outcome of reverting the fault (nil: clean). Actions
// with nothing to revert never call it, and their result has no cleanup.
func (t *Target) Reverted(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.reverted, t.revertErr = true, err
}

// SetState attaches what `pumba status` found on the target to its result.
func (t *Target) SetState(s *TargetState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state = s
}

// DeferResult is for actions that revert in a batch after fanout (pause,
// stop --restart). It keeps RunOnContainers from emitting the target's
// terminal result when the action returns nil; the action must call the
// returned func with the target's own revert error once it reverted the
// target. When the action returns an error, the result is emitted right away
// and the returned func does nothing.
func (t *Target) DeferResult() func(revertErr error) {
	t.mu.Lock()
	t.deferred = true
	t.mu.Unlock()
	return func(revertErr error) {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.reverted, t.revertErr = true, revertErr
		t.terminate(nil)
	}
}

// finish is called by RunOnContainers when the action returns.
func (t *Target) finish(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.deferred && err == nil {
		return
	}
	t.terminate(err)
}

// terminate emits target.result once. Callers hold t.mu.
func (t *Target) terminate(err error) {
	if t.terminated {
		return
	}
	t.terminated = true
	status, evErr := t.status(err)
	var cleanup *Cleanup
	if t.reverted && status != TargetPlanned {
		cleanup = &Cleanup{OK: t.revertErr == nil}
		if t.revertErr != nil {
			cleanup.Error = t.revertErr.Error()
		}
	}
	t.gp.Emit(&Event{Type: EventTargetResult, Result: &TargetResult{
		Tick:    t.tick,
		Target:  t.ref,
		Started: t.started,
		Ended:   time.Now().UTC(),
		Status:  status,
		Error:   evErr,
		Cleanup: cleanup,
		State:   t.state,
	}})
}

func (t *Target) status(err error) (TargetStatus, *EventError) {
	switch {
	case t.reverted && t.revertErr != nil:
		cause := t.revertErr
		if err != nil {
			cause = err // already wraps or joins the revert error
		}
		return TargetCleanupFailed, newEventError(CodeCleanupFailed, PhaseCleanup, cause)
	case err != nil:
		if coded, ok := errors.AsType[*codedError](err); ok {
			return TargetFailed, newEventError(coded.code, coded.phase, err)
		}
		return TargetFailed, newEventError(CodeCommandFailed, PhaseInject, err)
	case t.gp.DryRun:
		return TargetPlanned, nil
	default:
		return TargetOK, nil
	}
}

// skipTargets emits a skipped result for each target a serial run did not
// reach after an earlier target failed.
func skipTargets(gp *GlobalParams, tick int64, containers []*container.Container) {
	for _, c := range containers {
		now := time.Now().UTC()
		gp.Emit(&Event{Type: EventTargetResult, Result: &TargetResult{
			Tick:    tick,
			Target:  TargetRef{ID: c.ID(), Name: c.Name()},
			Started: now,
			Ended:   now,
			Status:  TargetSkipped,
		}})
	}
}
