package containerd

import (
	"context"
	"errors"
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/errdefs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWaitStressSidecarCleanupFailures(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"task", "container", "both", "not_found"} {
		for _, code := range []uint32{0, 1} {
			t.Run(stage+map[uint32]string{0: "/success", 1: "/failure"}[code], func(t *testing.T) {
				t.Parallel()
				taskErr, containerErr := errors.New("stress task deletion failed"), errors.New("stress snapshot deletion failed")
				var deleteTaskErr, deleteContainerErr error
				switch stage {
				case "task":
					deleteTaskErr = taskErr
				case "container":
					deleteContainerErr = containerErr
				case "both":
					deleteTaskErr, deleteContainerErr = taskErr, containerErr
				case "not_found":
					deleteTaskErr, deleteContainerErr = errdefs.ErrNotFound, errdefs.ErrNotFound
				}
				task := NewMockTask(t)
				task.EXPECT().Delete(mock.Anything).Return(nil, deleteTaskErr).Once()
				cntr := NewMockContainer(t)
				cntr.EXPECT().Delete(mock.Anything, mock.Anything).Return(deleteContainerErr).Once()
				waitCh := make(chan containerd.ExitStatus, 1)
				waitCh <- *containerd.NewExitStatus(code, time.Now(), nil)
				outCh, errCh := make(chan string, 1), make(chan error, 1)

				newTestClient(nil).waitStressSidecar(context.Background(), "stress-id", cntr, task, waitCh, outCh, errCh)

				if stage == "not_found" && code == 0 {
					assert.Equal(t, "stress-id", requireStressOutput(t, outCh, errCh, time.Second))
					return
				}
				err := requireStressError(t, outCh, errCh, time.Second)
				if stage == "task" || stage == "both" {
					assert.ErrorIs(t, err, taskErr)
				}
				if stage == "container" || stage == "both" {
					assert.ErrorIs(t, err, containerErr)
				}
				if code != 0 {
					assert.ErrorContains(t, err, "exited with code 1")
				}
			})
		}
	}
}

func TestCreateStressSidecarCleanupFailures(t *testing.T) { //nolint:paralleltest // mutates package-level cgroupReader
	for _, stage := range []string{"create_task", "wait", "start"} {
		t.Run(stage, func(t *testing.T) {
			operationErr, taskCleanupErr, containerCleanupErr := errors.New("stress startup failed"), errors.New("stress task deletion failed"), errors.New("stress container deletion failed")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cntr := NewMockContainer(t)
			if stage == "create_task" {
				cntr.EXPECT().NewTask(mock.Anything, mock.Anything).Run(func(context.Context, cio.Creator, ...containerd.NewTaskOpts) { cancel() }).Return(nil, operationErr).Once()
			} else {
				task := NewMockTask(t)
				cntr.EXPECT().NewTask(mock.Anything, mock.Anything).Return(task, nil).Once()
				if stage == "wait" {
					task.EXPECT().Wait(mock.Anything).Run(func(context.Context) { cancel() }).Return(nil, operationErr).Once()
				} else {
					task.EXPECT().Wait(mock.Anything).Return(make(<-chan containerd.ExitStatus), nil).Once()
					task.EXPECT().Start(mock.Anything).Run(func(context.Context) { cancel() }).Return(operationErr).Once()
				}
				task.EXPECT().Delete(mock.Anything).Run(func(cleanupCtx context.Context, _ ...containerd.ProcessDeleteOpts) {
					assert.NoError(t, cleanupCtx.Err())
					_, ok := cleanupCtx.Deadline()
					assert.True(t, ok)
				}).Return(nil, taskCleanupErr).Once()
			}
			cntr.EXPECT().Delete(mock.Anything, mock.Anything).Run(func(cleanupCtx context.Context, _ ...containerd.DeleteOpts) {
				assert.NoError(t, cleanupCtx.Err())
				_, ok := cleanupCtx.Deadline()
				assert.True(t, ok)
			}).Return(containerCleanupErr).Once()
			api := setupStressTarget(t, 42, "0::/system.slice/docker-c1.scope\n")
			setupStressSidecar(t, api, "stress-ng", cntr)

			result, err := newTestClient(api).StressContainer(ctx, stressReq(testContainer("c1"), []string{"--cpu", "1"}, "stress-ng", false, time.Second, false, false))

			require.ErrorIs(t, err, operationErr)
			assert.ErrorIs(t, err, containerCleanupErr)
			if stage != "create_task" {
				assert.ErrorIs(t, err, taskCleanupErr)
			}
			assert.Nil(t, result)
		})
	}
}

func TestStressDirectExecCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	proc := NewMockProcess(t)
	proc.EXPECT().Wait(mock.Anything).Return(make(<-chan containerd.ExitStatus), nil).Once()
	proc.EXPECT().Start(mock.Anything).Run(func(context.Context) { cancel() }).Return(nil).Once()
	proc.EXPECT().Delete(mock.Anything).Return(nil, nil).Once()
	task := newRunningTask(t)
	setupExec(task, proc)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))

	result, err := newTestClient(api).StressContainer(ctx, stressReq(testContainer("c1"), []string{"--cpu", "1"}, "", false, time.Hour, false, false))

	require.NoError(t, err)
	require.ErrorIs(t, requireStressError(t, result.Output, result.Errors, time.Second), context.Canceled)
}
