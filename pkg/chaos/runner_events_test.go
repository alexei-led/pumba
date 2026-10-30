package chaos_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// eventKey is the comparable part of a target event.
type eventKey struct {
	Type   chaos.EventType
	Tick   int64
	Target string
	Status chaos.TargetStatus
}

func keys(events []chaos.Event) []eventKey {
	out := make([]eventKey, 0, len(events))
	for _, ev := range events {
		switch ev.Type {
		case chaos.EventTargetApplied:
			out = append(out, eventKey{Type: ev.Type, Tick: ev.Applied.Tick, Target: ev.Applied.Target.ID})
		case chaos.EventTargetResult:
			out = append(out, eventKey{Type: ev.Type, Tick: ev.Result.Tick, Target: ev.Result.Target.ID, Status: ev.Result.Status})
		default:
			out = append(out, eventKey{Type: ev.Type})
		}
	}
	return out
}

func applied(tick int64, id string) eventKey {
	return eventKey{Type: chaos.EventTargetApplied, Tick: tick, Target: id}
}

func result(tick int64, id string, s chaos.TargetStatus) eventKey {
	return eventKey{Type: chaos.EventTargetResult, Tick: tick, Target: id, Status: s}
}

func expectList(client *container.MockClient, ids ...string) {
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).
		Return(makeContainers(ids...), nil).Once()
}

// applyAndRevert is a ContainerAction that applies, then reverts with revertErr,
// and fails with err.
func applyAndRevert(revertErr, err error) chaos.ContainerAction {
	return func(_ context.Context, _ *container.Container, t *chaos.Target) error {
		t.Applied()
		t.Reverted(revertErr)
		return err
	}
}

func TestRunOnContainersEvents(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name     string
		ids      []string
		parallel bool
		dryRun   bool
		fn       func(id string) chaos.ContainerAction
		wantErr  bool
		want     []eventKey
	}{
		{
			name: "serial ok",
			ids:  []string{"a", "b"},
			fn:   func(string) chaos.ContainerAction { return applyAndRevert(nil, nil) },
			want: []eventKey{applied(1, "a"), result(1, "a", chaos.TargetOK), applied(1, "b"), result(1, "b", chaos.TargetOK)},
		},
		{
			name: "serial failure skips the rest",
			ids:  []string{"a", "b", "c"},
			fn: func(id string) chaos.ContainerAction {
				if id == "a" {
					return func(context.Context, *container.Container, *chaos.Target) error { return boom }
				}
				return applyAndRevert(nil, nil)
			},
			wantErr: true,
			want:    []eventKey{result(1, "a", chaos.TargetFailed), result(1, "b", chaos.TargetSkipped), result(1, "c", chaos.TargetSkipped)},
		},
		{
			name:    "cleanup failure",
			ids:     []string{"a"},
			fn:      func(string) chaos.ContainerAction { return applyAndRevert(boom, fmt.Errorf("stopping: %w", boom)) },
			wantErr: true,
			want:    []eventKey{applied(1, "a"), result(1, "a", chaos.TargetCleanupFailed)},
		},
		{
			name:   "dry-run is planned and never applied",
			ids:    []string{"a"},
			dryRun: true,
			fn:     func(string) chaos.ContainerAction { return applyAndRevert(nil, nil) },
			want:   []eventKey{result(1, "a", chaos.TargetPlanned)},
		},
		{
			name: "one-shot without revert",
			ids:  []string{"a"},
			fn: func(string) chaos.ContainerAction {
				return func(_ context.Context, _ *container.Container, t *chaos.Target) error { t.Applied(); return nil }
			},
			want: []eventKey{applied(1, "a"), result(1, "a", chaos.TargetOK)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := container.NewMockClient(t)
			expectList(client, tt.ids...)
			sink := &recordSink{}
			gp := &chaos.GlobalParams{RunID: "r1", Action: "test", Events: sink, DryRun: tt.dryRun}
			err := chaos.RunOnContainers(context.Background(), client, gp, 0, false, tt.parallel,
				func(ctx context.Context, c *container.Container, target *chaos.Target) error {
					return tt.fn(c.ID())(ctx, c, target)
				})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, keys(sink.all()))
		})
	}
}

func TestRunOnContainersEventPayloads(t *testing.T) {
	client := container.NewMockClient(t)
	expectList(client, "a")
	sink := &recordSink{}
	gp := &chaos.GlobalParams{RunID: "r1", Action: "netem delay", Events: sink}
	boom := errors.New("boom")
	err := chaos.RunOnContainers(context.Background(), client, gp, 0, false, true, applyAndRevert(boom, fmt.Errorf("stopping netem: %w", boom)))
	require.Error(t, err)

	results := sink.ofType(chaos.EventTargetResult)
	require.Len(t, results, 1)
	ev := results[0]
	assert.Equal(t, "r1", ev.RunID)
	assert.Equal(t, "netem delay", ev.Action)
	assert.Equal(t, chaos.TargetRef{ID: "a", Name: "a"}, ev.Result.Target)
	assert.Equal(t, &chaos.Cleanup{OK: false, Error: "boom"}, ev.Result.Cleanup)
	assert.Equal(t, &chaos.EventError{Code: chaos.CodeCleanupFailed, Phase: chaos.PhaseCleanup, Message: "stopping netem: boom"}, ev.Result.Error)
	assert.False(t, ev.Result.Ended.Before(ev.Result.Started))
}

func TestRunOnContainersParallelAppliedBeforeResultPerTarget(t *testing.T) {
	client := container.NewMockClient(t)
	ids := []string{"a", "b", "c", "d"}
	expectList(client, ids...)
	sink := &recordSink{}
	gp := &chaos.GlobalParams{Events: sink}
	require.NoError(t, chaos.RunOnContainers(context.Background(), client, gp, 0, false, true, applyAndRevert(nil, nil)))

	seen := map[string]chaos.EventType{}
	for _, k := range keys(sink.all()) {
		if k.Type == chaos.EventTargetResult {
			assert.Equal(t, chaos.EventTargetApplied, seen[k.Target], "target %s: result before applied", k.Target)
		}
		seen[k.Target] = k.Type
	}
	assert.Len(t, sink.ofType(chaos.EventTargetResult), len(ids))
}

func TestRunOnContainersParallelFailureReportsEveryTarget(t *testing.T) {
	client := container.NewMockClient(t)
	expectList(client, "a", "b")
	sink := &recordSink{}
	gp := &chaos.GlobalParams{Events: sink}
	err := chaos.RunOnContainers(context.Background(), client, gp, 0, false, true,
		func(_ context.Context, c *container.Container, t *chaos.Target) error {
			if c.ID() == "b" {
				return errors.New("fail-b")
			}
			t.Applied()
			return nil
		})
	require.Error(t, err)
	statuses := map[string]chaos.TargetStatus{}
	for _, ev := range sink.ofType(chaos.EventTargetResult) {
		statuses[ev.Result.Target.ID] = ev.Result.Status
	}
	assert.Equal(t, map[string]chaos.TargetStatus{"a": chaos.TargetOK, "b": chaos.TargetFailed}, statuses)
}

func TestRunOnContainersRandomReportsOnlyThePick(t *testing.T) {
	client := container.NewMockClient(t)
	expectList(client, "a", "b", "c")
	sink := &recordSink{}
	gp := &chaos.GlobalParams{Events: sink}
	require.NoError(t, chaos.RunOnContainers(context.Background(), client, gp, 0, true, true, applyAndRevert(nil, nil)))
	assert.Len(t, sink.ofType(chaos.EventTargetApplied), 1)
	assert.Len(t, sink.ofType(chaos.EventTargetResult), 1)
}

func TestRunOnContainersNoTargetEvents(t *testing.T) {
	tests := []struct {
		name    string
		ids     []string
		cap     int
		listErr error
		wantErr error
	}{
		{name: "no targets"},
		{name: "max-targets refusal", ids: []string{"a", "b"}, cap: 1, wantErr: chaos.ErrMaxTargetsExceeded},
		{name: "list failure", listErr: errors.New("socket down")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := container.NewMockClient(t)
			client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).
				Return(makeContainers(tt.ids...), tt.listErr).Once()
			sink := &recordSink{}
			gp := &chaos.GlobalParams{MaxTargets: tt.cap, Events: sink}
			err := chaos.RunOnContainers(context.Background(), client, gp, 0, false, true,
				func(context.Context, *container.Container, *chaos.Target) error {
					t.Fatal("no target may run")
					return nil
				})
			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.listErr != nil:
				require.ErrorIs(t, err, tt.listErr)
				assert.EqualError(t, err, "listing containers: failed to list containers: socket down", "message unchanged")
				o := chaos.NewOutcome(chaos.Counts{}, err, false)
				assert.Equal(t, chaos.CodeRuntimeUnavailable, o.Error.Code)
				assert.Equal(t, chaos.PhaseSelect, o.Error.Phase)
			default:
				require.NoError(t, err)
			}
			assert.Empty(t, sink.all())
		})
	}
}

func TestRunOnContainersTickPerCall(t *testing.T) {
	client := container.NewMockClient(t)
	expectList(client, "a")
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).Return(nil, nil).Once()
	expectList(client, "a")
	sink := &recordSink{}
	gp := &chaos.GlobalParams{Events: sink}
	for range 3 {
		require.NoError(t, chaos.RunOnContainers(context.Background(), client, gp, 0, false, false, applyAndRevert(nil, nil)))
	}
	assert.Equal(t, []eventKey{
		applied(1, "a"), result(1, "a", chaos.TargetOK),
		applied(3, "a"), result(3, "a", chaos.TargetOK),
	}, keys(sink.all()), "a tick with no targets still counts")
}

func TestRunOnContainersCanceledTargetStillReportsAfterCleanup(t *testing.T) {
	client := container.NewMockClient(t)
	expectList(client, "a")
	sink := &recordSink{}
	gp := &chaos.GlobalParams{Events: sink}
	ctx, cancel := context.WithCancel(context.Background())
	err := chaos.RunOnContainers(ctx, client, gp, 0, false, true,
		func(ctx context.Context, _ *container.Container, target *chaos.Target) error {
			target.Applied()
			cancel()
			<-ctx.Done()
			assert.Empty(t, sink.ofType(chaos.EventTargetResult), "no result before cleanup")
			target.Reverted(nil)
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, []eventKey{applied(1, "a"), result(1, "a", chaos.TargetOK)}, keys(sink.all()))
}

func TestTargetDeferResult(t *testing.T) {
	boom := errors.New("unpause failed")
	tests := []struct {
		name        string
		actionErr   error
		revertErr   error
		wantStatus  chaos.TargetStatus
		wantCleanup *chaos.Cleanup
	}{
		{name: "reported after revert", wantStatus: chaos.TargetOK, wantCleanup: &chaos.Cleanup{OK: true}},
		{name: "revert failure", revertErr: boom, wantStatus: chaos.TargetCleanupFailed, wantCleanup: &chaos.Cleanup{Error: "unpause failed"}},
		{name: "action error reports at once", actionErr: errors.New("pause failed"), wantStatus: chaos.TargetFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := container.NewMockClient(t)
			expectList(client, "a")
			sink := &recordSink{}
			gp := &chaos.GlobalParams{Events: sink}
			var done func(error)
			err := chaos.RunOnContainers(context.Background(), client, gp, 0, false, false,
				func(_ context.Context, _ *container.Container, t *chaos.Target) error {
					done = t.DeferResult()
					return tt.actionErr
				})
			require.Equal(t, tt.actionErr, err)
			if tt.actionErr == nil {
				assert.Empty(t, sink.ofType(chaos.EventTargetResult), "deferred result must wait for the revert")
			}
			done(tt.revertErr)
			done(nil) // a second call is a no-op
			results := sink.ofType(chaos.EventTargetResult)
			require.Len(t, results, 1)
			assert.Equal(t, tt.wantStatus, results[0].Result.Status)
			assert.Equal(t, tt.wantCleanup, results[0].Result.Cleanup)
		})
	}
}

func TestTargetConcurrentReports(t *testing.T) {
	client := container.NewMockClient(t)
	expectList(client, "a")
	sink := &recordSink{}
	gp := &chaos.GlobalParams{Events: sink}
	require.NoError(t, chaos.RunOnContainers(context.Background(), client, gp, 0, false, true,
		func(_ context.Context, _ *container.Container, t *chaos.Target) error {
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(t.Applied)
			}
			wg.Wait()
			return nil
		}))
	assert.Len(t, sink.ofType(chaos.EventTargetApplied), 1, "applied is emitted once")
}

func TestRunOnContainersSerialStopsStartingTargetsAfterCancel(t *testing.T) {
	client := container.NewMockClient(t)
	expectList(client, "a", "b", "c")
	sink := &recordSink{}
	gp := &chaos.GlobalParams{Events: sink}
	ctx, cancel := context.WithCancel(context.Background())
	var called []string
	err := chaos.RunOnContainers(ctx, client, gp, 0, false, false,
		func(_ context.Context, c *container.Container, target *chaos.Target) error {
			called = append(called, c.ID())
			target.Applied()
			target.Reverted(nil)
			cancel()
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, called, "no target may start after cancellation")
	assert.Equal(t, []eventKey{
		applied(1, "a"), result(1, "a", chaos.TargetOK),
		result(1, "b", chaos.TargetSkipped), result(1, "c", chaos.TargetSkipped),
	}, keys(sink.all()))
}
