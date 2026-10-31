package containerd

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/errdefs"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestExecFaultTaskOutputCompletion(t *testing.T) {
	t.Parallel()
	task := NewMockTask(t)
	setupExec(task, newSuccessProcess(t))

	_, _, err := execFaultTaskOutput(context.Background(), task, &specs.Process{Args: []string{"true"}}, "exec-id", "fault")

	require.NoError(t, err)
}

func TestExecFaultTaskOutputAlreadyCanceled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := execFaultTaskOutput(ctx, NewMockTask(t), &specs.Process{}, "exec-id", "fault")

	require.ErrorIs(t, err, context.Canceled)
}

func TestExecFaultTaskOutputInternalDeadline(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"none", "wait", "kill", "delete", "closed_wait", "not_found"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			terminationErr, deletionErr := errors.New("termination failed"), errors.New("exec deletion failed")
			proc := NewMockProcess(t)
			exitCh := make(chan containerd.ExitStatus, 1)
			proc.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil).Once()
			proc.EXPECT().Start(mock.Anything).Return(nil).Once()
			var waitErr, killErr, deleteErr error
			cleanupExitCh := exitCh
			switch failure {
			case "wait":
				waitErr = terminationErr
			case "kill":
				killErr = terminationErr
			case "delete":
				deleteErr = deletionErr
			case "closed_wait":
				cleanupExitCh = make(chan containerd.ExitStatus)
				close(cleanupExitCh)
			case "not_found":
				killErr, deleteErr = errdefs.ErrNotFound, errdefs.ErrNotFound
			}
			proc.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(cleanupExitCh), waitErr).Once()
			// No KillOpts: signal only this exec, never all processes in the target.
			proc.EXPECT().Kill(mock.Anything, syscall.SIGKILL).Run(func(cleanupCtx context.Context, _ syscall.Signal, _ ...containerd.KillOpts) {
				assert.NoError(t, cleanupCtx.Err(), "termination must survive the expired execution deadline")
				_, ok := cleanupCtx.Deadline()
				assert.True(t, ok, "termination must be bounded")
				if failure == "none" || failure == "delete" {
					exitCh <- *containerd.NewExitStatus(137, time.Now(), nil)
				}
			}).Return(killErr).Once()
			proc.EXPECT().Delete(mock.Anything).Run(func(cleanupCtx context.Context, _ ...containerd.ProcessDeleteOpts) {
				assert.NoError(t, cleanupCtx.Err())
				if failure == "none" || failure == "delete" {
					assert.Empty(t, exitCh, "exec must exit before it is deleted and rollback may proceed")
				}
			}).Return(nil, deleteErr).Once()
			task := NewMockTask(t)
			setupExec(task, proc)

			out, errOut, err := execFaultTaskOutputWithTimeout(context.Background(), task, &specs.Process{}, "exec-id", "fault", 10*time.Millisecond)

			require.ErrorIs(t, err, context.DeadlineExceeded)
			assert.Empty(t, out)
			assert.Empty(t, errOut)
			switch failure {
			case "wait", "kill":
				assert.ErrorIs(t, err, terminationErr)
				assert.ErrorContains(t, err, "fault subprocesses may remain")
			case "delete":
				assert.ErrorIs(t, err, deletionErr)
			case "closed_wait":
				assert.ErrorContains(t, err, "wait channel closed without exit status")
			}
		})
	}
}

func TestTerminateFaultExecWaitIsBounded(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	proc := NewMockProcess(t)
	proc.EXPECT().Wait(mock.Anything).Return(make(<-chan containerd.ExitStatus), nil).Once()
	proc.EXPECT().Kill(mock.Anything, syscall.SIGKILL).Return(nil).Once()

	err := terminateFaultExec(ctx, proc)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.ErrorContains(t, err, "waiting for fault exec after SIGKILL")
}
