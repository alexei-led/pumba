package stress

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func completionCommand(t *testing.T, duration time.Duration, start func(context.Context) *container.StressResult, sink chaos.EventSink) chaos.Command {
	t.Helper()
	client := container.NewMockClient(t)
	target := testContainer("target-id", "target")
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).Return([]*container.Container{target}, nil)
	client.EXPECT().StressContainer(mock.Anything, &container.StressRequest{Container: target, Stressors: []string{"--cpu", "1"}, Duration: duration}).RunAndReturn(func(ctx context.Context, _ *container.StressRequest) (*container.StressResult, error) {
		return start(ctx), nil
	})
	return NewStressCommand(client, &chaos.GlobalParams{Events: sink}, "", false, "--cpu 1", duration, 0, false)
}

func TestStressCommand_DrainsClosedChannels(t *testing.T) {
	failure := errors.New("workload failed after output closed")
	for range 100 {
		output := make(chan string)
		errs := make(chan error, 1)
		close(output)
		errs <- failure
		close(errs)
		cmd := completionCommand(t, time.Second, func(context.Context) *container.StressResult {
			return &container.StressResult{Output: output, Errors: errs}
		}, nil)
		require.ErrorIs(t, cmd.Run(context.Background(), false), failure)
	}
}

func TestStressCommand_WaitsForCleanup(t *testing.T) {
	for _, mode := range []string{"natural", "duration", "SIGTERM"} {
		t.Run(mode, func(t *testing.T) {
			output := make(chan string, 1)
			errs := make(chan error, 1)
			cleanup := make(chan struct{})
			requested := make(chan struct{})
			sink := &recordSink{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			duration := time.Hour
			if mode == "duration" {
				duration = time.Millisecond
			}
			cmd := completionCommand(t, duration, func(workCtx context.Context) *container.StressResult {
				output <- "done"
				close(output)
				go func() {
					if mode != "natural" {
						<-workCtx.Done()
					}
					close(requested)
					<-cleanup
					errs <- errors.New("late workload failure")
					close(errs)
				}()
				return &container.StressResult{Output: output, Errors: errs}
			}, sink)
			done := make(chan error, 1)
			go func() { done <- cmd.Run(ctx, false) }()
			if mode == "SIGTERM" {
				cancel()
			}
			select {
			case <-requested:
			case <-time.After(time.Second):
				t.Fatal("shutdown not requested")
			}
			select {
			case err := <-done:
				t.Fatalf("finished before cleanup: %v", err)
			case <-time.After(10 * time.Millisecond):
			}
			sink.mu.Lock()
			eventCount := len(sink.events)
			sink.mu.Unlock()
			require.Equal(t, 1, eventCount)
			close(cleanup)
			select {
			case err := <-done:
				require.ErrorContains(t, err, "late workload failure")
			case <-time.After(time.Second):
				t.Fatal("cleanup did not finish")
			}
		})
	}
}

func TestStressCommand_ClosedErrorsWithOutput(t *testing.T) {
	for range 100 {
		output, errs := make(chan string, 1), make(chan error)
		output <- "done"
		close(output)
		close(errs)
		sink := &recordSink{}
		cmd := completionCommand(t, time.Second, func(context.Context) *container.StressResult {
			return &container.StressResult{Output: output, Errors: errs}
		}, sink)
		require.NoError(t, cmd.Run(context.Background(), false))
		require.Equal(t, &chaos.Cleanup{OK: true}, sink.events[1].Result.Cleanup)
	}
}

func TestWaitStress_BoundedCleanupPreservesErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel()
	failure, cleanupFailure := errors.New("genuine workload failure"), errors.New("cleanup failure")
	errs := make(chan error, 2)
	errs <- failure
	errs <- &container.StressCleanupError{Err: cleanupFailure}
	resultErr, cleanupErr := waitStress(ctx, &container.StressResult{Output: make(chan string), Errors: errs}, time.Hour, time.Millisecond, cancel)
	require.ErrorIs(t, resultErr, failure)
	require.ErrorIs(t, resultErr, cleanupFailure)
	require.ErrorIs(t, cleanupErr, cleanupFailure)
	require.ErrorIs(t, cleanupErr, context.DeadlineExceeded)
}

func TestStressCommand_StartupCleanupFailure(t *testing.T) {
	client := container.NewMockClient(t)
	target := testContainer("target-id", "target")
	failure, cleanup := errors.New("start failed"), errors.New("remove failed")
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).Return([]*container.Container{target}, nil)
	client.EXPECT().StressContainer(mock.Anything, &container.StressRequest{Container: target, Stressors: []string{"--cpu", "1"}, Duration: time.Second}).Return(nil, errors.Join(failure, &container.StressCleanupError{Err: cleanup}))
	sink := &recordSink{}
	cmd := NewStressCommand(client, &chaos.GlobalParams{Events: sink}, "", false, "--cpu 1", time.Second, 0, false)
	err := cmd.Run(context.Background(), false)
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, err, cleanup)
	require.Len(t, sink.events, 1)
	require.Equal(t, chaos.TargetCleanupFailed, sink.events[0].Result.Status)
}
