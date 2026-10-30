package lifecycle

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// orderLog is an event sink that also records runtime calls, so tests can
// check the order of reverts and target events. Keyed by name: test containers have no IDs.
type orderLog struct {
	mu      sync.Mutex
	entries []string
	results map[string]chaos.TargetResult
}

func (l *orderLog) add(entry string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

func (l *orderLog) Emit(ev *chaos.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch ev.Type {
	case chaos.EventTargetApplied:
		l.entries = append(l.entries, "applied:"+ev.Applied.Target.Name)
	case chaos.EventTargetResult:
		l.entries = append(l.entries, "result:"+ev.Result.Target.Name)
		if l.results == nil {
			l.results = map[string]chaos.TargetResult{}
		}
		l.results[ev.Result.Target.Name] = *ev.Result
	}
}

func TestBatchRevertReportsEachTargetAfterItsOwnRevert(t *testing.T) {
	unpauseErr := errors.New("unpause failed")
	for _, action := range []string{"pause", "stop --restart"} {
		t.Run(action, func(t *testing.T) {
			client := container.NewMockClient(t)
			log := &orderLog{}
			gp := &chaos.GlobalParams{Names: []string{"c0", "c1"}, Events: log}
			targets := container.CreateTestContainers(2)
			client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).Return(targets, nil).Once()
			revertResult := func(c *container.Container) error {
				log.add("revert:" + c.Name())
				if c.Name() == targets[1].Name() {
					return unpauseErr
				}
				return nil
			}
			var command chaos.Command
			if action == "pause" {
				client.EXPECT().PauseContainer(mock.Anything, mock.AnythingOfType("*container.Container"), false).Return(nil).Times(2)
				client.EXPECT().UnpauseContainer(mock.Anything, mock.AnythingOfType("*container.Container"), false).
					RunAndReturn(func(_ context.Context, c *container.Container, _ bool) error { return revertResult(c) }).Times(2)
				command = NewPauseCommand(client, gp, 10*time.Millisecond, 0)
			} else {
				client.EXPECT().StopContainer(mock.Anything, mock.AnythingOfType("*container.Container"), DeafultWaitTime, false).Return(nil).Times(2)
				client.EXPECT().StartContainer(mock.Anything, mock.AnythingOfType("*container.Container"), false).
					RunAndReturn(func(_ context.Context, c *container.Container, _ bool) error { return revertResult(c) }).Times(2)
				command = NewStopCommand(client, gp, true, 10*time.Millisecond, 0, 0)
			}

			err := command.Run(context.Background(), false)

			require.ErrorIs(t, err, unpauseErr)
			a, b := targets[0].Name(), targets[1].Name()
			assert.Equal(t, []string{
				"applied:" + a, "applied:" + b,
				"revert:" + a, "result:" + a,
				"revert:" + b, "result:" + b,
			}, log.entries)
			assert.Equal(t, chaos.TargetOK, log.results[a].Status)
			assert.Equal(t, &chaos.Cleanup{OK: true}, log.results[a].Cleanup)
			assert.Equal(t, chaos.TargetCleanupFailed, log.results[b].Status)
			assert.False(t, log.results[b].Cleanup.OK)
		})
	}
}

func TestBatchRevertAfterCancelStillReportsAfterRevert(t *testing.T) {
	client := container.NewMockClient(t)
	log := &orderLog{}
	gp := &chaos.GlobalParams{Names: []string{"c0"}, Events: log}
	targets := container.CreateTestContainers(1)
	ctx, cancel := context.WithCancel(context.Background())
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).Return(targets, nil).Once()
	client.EXPECT().PauseContainer(mock.Anything, targets[0], false).
		RunAndReturn(func(context.Context, *container.Container, bool) error { cancel(); return nil }).Once()
	client.EXPECT().UnpauseContainer(mock.Anything, targets[0], false).
		RunAndReturn(func(ctx context.Context, c *container.Container, _ bool) error {
			assert.NoError(t, ctx.Err(), "unpause runs on a detached context")
			log.add("revert:" + c.Name())
			return nil
		}).Once()

	require.NoError(t, NewPauseCommand(client, gp, time.Hour, 0).Run(ctx, false))
	id := targets[0].Name()
	assert.Equal(t, []string{"applied:" + id, "revert:" + id, "result:" + id}, log.entries)
}

func TestBatchRevertAfterDurationSurvivesCancelDuringRevert(t *testing.T) {
	for _, action := range []string{"pause", "stop --restart"} {
		t.Run(action, func(t *testing.T) {
			client := container.NewMockClient(t)
			log := &orderLog{}
			gp := &chaos.GlobalParams{Names: []string{"c0", "c1"}, Events: log}
			targets := container.CreateTestContainers(2)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			revert := func(revertCtx context.Context, c *container.Container) error {
				if c == targets[0] {
					cancel() // SIGTERM or --max-duration arrives mid-revert
				}
				return revertCtx.Err()
			}
			client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).Return(targets, nil).Once()
			var command chaos.Command
			if action == "pause" {
				client.EXPECT().PauseContainer(mock.Anything, mock.AnythingOfType("*container.Container"), false).Return(nil).Times(2)
				client.EXPECT().UnpauseContainer(mock.Anything, mock.AnythingOfType("*container.Container"), false).
					RunAndReturn(func(ctx context.Context, c *container.Container, _ bool) error { return revert(ctx, c) }).Times(2)
				command = NewPauseCommand(client, gp, time.Millisecond, 0)
			} else {
				client.EXPECT().StopContainer(mock.Anything, mock.AnythingOfType("*container.Container"), DeafultWaitTime, false).Return(nil).Times(2)
				client.EXPECT().StartContainer(mock.Anything, mock.AnythingOfType("*container.Container"), false).
					RunAndReturn(func(ctx context.Context, c *container.Container, _ bool) error { return revert(ctx, c) }).Times(2)
				command = NewStopCommand(client, gp, true, time.Millisecond, 0, 0)
			}

			require.NoError(t, command.Run(ctx, false))
			for _, c := range targets {
				assert.Equal(t, chaos.TargetOK, log.results[c.Name()].Status)
			}
		})
	}
}

func TestPauseFailureSkipsRestAndRevertsPaused(t *testing.T) {
	client := container.NewMockClient(t)
	log := &orderLog{}
	gp := &chaos.GlobalParams{Names: []string{"c0", "c1", "c2"}, Events: log}
	targets := container.CreateTestContainers(3)
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).Return(targets, nil).Once()
	client.EXPECT().PauseContainer(mock.Anything, targets[0], false).Return(nil).Once()
	client.EXPECT().PauseContainer(mock.Anything, targets[1], false).Return(errors.New("pause failed")).Once()
	client.EXPECT().UnpauseContainer(mock.Anything, targets[0], false).Return(nil).Once()

	require.Error(t, NewPauseCommand(client, gp, time.Millisecond, 0).Run(context.Background(), false))
	assert.Equal(t, chaos.TargetOK, log.results[targets[0].Name()].Status)
	assert.Equal(t, chaos.TargetFailed, log.results[targets[1].Name()].Status)
	assert.Equal(t, chaos.TargetSkipped, log.results[targets[2].Name()].Status)
}

func TestStopWithoutRestartReportsAtOnce(t *testing.T) {
	client := container.NewMockClient(t)
	log := &orderLog{}
	gp := &chaos.GlobalParams{Names: []string{"c0"}, Events: log}
	targets := container.CreateTestContainers(1)
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).Return(targets, nil).Once()
	client.EXPECT().StopContainer(mock.Anything, targets[0], DeafultWaitTime, false).Return(nil).Once()

	require.NoError(t, NewStopCommand(client, gp, false, time.Hour, 0, 0).Run(context.Background(), false))
	id := targets[0].Name()
	assert.Equal(t, []string{"applied:" + id, "result:" + id}, log.entries)
	assert.Nil(t, log.results[id].Cleanup, "stop without restart leaves nothing to revert")
}

func TestInterruptedActionIsStillReverted(t *testing.T) {
	actionErr := context.Canceled
	for _, action := range []string{"pause", "stop --restart"} {
		t.Run(action, func(t *testing.T) {
			client := container.NewMockClient(t)
			log := &orderLog{}
			gp := &chaos.GlobalParams{Names: []string{"c0"}, Events: log}
			targets := container.CreateTestContainers(1)
			ctx, cancel := context.WithCancel(context.Background())
			client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).Return(targets, nil).Once()
			interrupt := func(context.Context, *container.Container, ...any) error { cancel(); return actionErr }
			revert := func(revertCtx context.Context, c *container.Container, _ bool) error {
				assert.NoError(t, revertCtx.Err(), "revert runs on a detached context")
				log.add("revert:" + c.Name())
				return nil
			}
			var command chaos.Command
			if action == "pause" {
				client.EXPECT().PauseContainer(mock.Anything, targets[0], false).
					RunAndReturn(func(ctx context.Context, c *container.Container, _ bool) error { return interrupt(ctx, c) }).Once()
				client.EXPECT().UnpauseContainer(mock.Anything, targets[0], false).RunAndReturn(revert).Once()
				command = NewPauseCommand(client, gp, time.Hour, 0)
			} else {
				client.EXPECT().StopContainer(mock.Anything, targets[0], DeafultWaitTime, false).
					RunAndReturn(func(ctx context.Context, c *container.Container, _ int, _ bool) error { return interrupt(ctx, c) }).Once()
				client.EXPECT().StartContainer(mock.Anything, targets[0], false).RunAndReturn(revert).Once()
				command = NewStopCommand(client, gp, true, time.Hour, 0, 0)
			}

			err := command.Run(ctx, false)

			require.ErrorIs(t, err, actionErr)
			name := targets[0].Name()
			assert.Equal(t, []string{"revert:" + name, "result:" + name}, log.entries)
			assert.Equal(t, chaos.TargetFailed, log.results[name].Status)
			assert.Equal(t, &chaos.Cleanup{OK: true}, log.results[name].Cleanup)
		})
	}
}

func TestEachRevertGetsItsOwnDeadline(t *testing.T) {
	targets := container.CreateTestContainers(2)
	held := make([]heldTarget, len(targets))
	for i, c := range targets {
		held[i] = heldTarget{c: c, done: func(error) {}}
	}
	var deadlines []time.Time
	err := revertAfter(context.Background(), 0, true, held, func(ctx context.Context, _ *container.Container) error {
		d, ok := ctx.Deadline()
		require.True(t, ok)
		deadlines = append(deadlines, d)
		time.Sleep(20 * time.Millisecond)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, deadlines, 2)
	assert.Greater(t, deadlines[1].Sub(deadlines[0]), 10*time.Millisecond, "second revert must not share the first one's deadline")
}
