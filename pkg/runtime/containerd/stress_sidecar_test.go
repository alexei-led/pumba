package containerd

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"testing"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/errdefs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
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
			var cleanup *ctr.StressCleanupError
			require.ErrorAs(t, err, &cleanup)
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
	for _, mode := range []string{"cancel", "duration", "grpc_cancel", "cleanup_failure", "natural_failure", "late_failure", "late_success", "late_killed"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "duration" {
				var stop context.CancelFunc
				ctx, stop = context.WithTimeout(ctx, 10*time.Millisecond)
				defer stop()
			}
			cleanupErr := errors.New("owned exec delete failed")
			proc := NewMockProcess(t)
			running, reaped := make(chan containerd.ExitStatus, 1), make(chan containerd.ExitStatus, 1)
			if mode == "natural_failure" {
				running <- *containerd.NewExitStatus(7, time.Now(), nil)
			}
			proc.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(running), nil).Once()
			if mode != "natural_failure" {
				proc.EXPECT().Wait(mock.Anything).Run(func(cleanupCtx context.Context) {
					assert.NoError(t, cleanupCtx.Err())
					_, ok := cleanupCtx.Deadline()
					assert.True(t, ok)
				}).Return((<-chan containerd.ExitStatus)(reaped), nil).Once()
			}
			proc.EXPECT().Start(mock.Anything).Run(func(context.Context) {
				if mode != "duration" {
					cancel()
				}
				if mode == "grpc_cancel" {
					running <- *containerd.NewExitStatus(0, time.Now(), grpcstatus.Error(codes.Canceled, "wait canceled"))
				}
			}).Return(nil).Once()
			var killErr error
			if mode == "late_failure" || mode == "late_success" || mode == "late_killed" {
				killErr = errdefs.ErrNotFound
			}
			if mode != "natural_failure" {
				proc.EXPECT().Kill(mock.Anything, syscall.SIGKILL).Run(func(context.Context, syscall.Signal, ...containerd.KillOpts) {
					if killErr == nil {
						reaped <- *containerd.NewExitStatus(137, time.Now(), nil)
					}
				}).Return(killErr).Once()
			}
			var deleteErr error
			if mode == "cleanup_failure" {
				deleteErr = cleanupErr
			}
			code := uint32(137)
			if mode == "late_failure" || mode == "natural_failure" {
				code = 7
			} else if mode == "late_success" {
				code = 0
			}
			proc.EXPECT().Delete(mock.Anything).Run(func(cleanupCtx context.Context, _ ...containerd.ProcessDeleteOpts) {
				assert.NoError(t, cleanupCtx.Err())
				_, ok := cleanupCtx.Deadline()
				assert.True(t, ok)
			}).Return(containerd.NewExitStatus(code, time.Now(), nil), deleteErr).Once()
			if mode == "natural_failure" {
				proc.EXPECT().IO().Return(newHeldIO()).Once()
			}
			task := newRunningTask(t)
			setupExec(task, proc)
			api := NewMockapiClient(t)
			setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))
			result, err := newTestClient(api).StressContainer(ctx, stressReq(testContainer("c1"), []string{"--cpu", "1"}, "", false, time.Hour, false, false))
			require.NoError(t, err)
			require.Empty(t, result.SidecarID)
			switch mode {
			case "cleanup_failure":
				err = requireStressError(t, result.Output, result.Errors, time.Second)
				require.ErrorIs(t, err, cleanupErr)
				var cleanup *ctr.StressCleanupError
				require.ErrorAs(t, err, &cleanup)
			case "natural_failure", "late_failure":
				require.ErrorContains(t, requireStressError(t, result.Output, result.Errors, time.Second), "exited with code 7")
			case "late_killed":
				require.ErrorContains(t, requireStressError(t, result.Output, result.Errors, time.Second), "exited with code 137")
			default:
				requireStressOutput(t, result.Output, result.Errors, time.Second)
			}
		})
	}
}

func TestWaitStressSidecarCancellationRetainsFailures(t *testing.T) {
	t.Parallel()
	for _, code := range []uint32{1, 137} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			task, cntr := NewMockTask(t), NewMockContainer(t)
			cleanupErr := errors.New("sidecar cleanup failed")
			task.EXPECT().Delete(mock.Anything).Run(func(ctx context.Context, _ ...containerd.ProcessDeleteOpts) { assert.NoError(t, ctx.Err()) }).Return(nil, cleanupErr).Once()
			cntr.EXPECT().Delete(mock.Anything, mock.Anything).Return(nil).Once()
			exited := make(chan containerd.ExitStatus, 1)
			exited <- *containerd.NewExitStatus(code, time.Now(), nil)
			output, errs := make(chan string, 1), make(chan error, 1)
			newTestClient(nil).waitStressSidecar(ctx, "owned-stress", cntr, task, exited, output, errs)
			err := requireStressError(t, output, errs, time.Second)
			require.ErrorIs(t, err, cleanupErr)
			require.ErrorContains(t, err, fmt.Sprintf("exited with code %d", code))
			var cleanup *ctr.StressCleanupError
			require.ErrorAs(t, err, &cleanup)
		})
	}
}

func TestWaitStressSidecarFailedKillDoesNotHideExit(t *testing.T) {
	t.Parallel()
	for _, killErr := range []error{errdefs.ErrNotFound, errors.New("kill failed")} {
		t.Run(killErr.Error(), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			task, cntr := NewMockTask(t), NewMockContainer(t)
			exited := make(chan containerd.ExitStatus, 1)
			task.EXPECT().Kill(mock.Anything, syscall.SIGKILL).Run(func(context.Context, syscall.Signal, ...containerd.KillOpts) {
				exited <- *containerd.NewExitStatus(137, time.Now(), nil)
			}).Return(killErr).Once()
			task.EXPECT().Delete(mock.Anything).Return(nil, nil).Once()
			cntr.EXPECT().Delete(mock.Anything, mock.Anything).Return(nil).Once()
			output, errs := make(chan string, 1), make(chan error, 1)
			newTestClient(nil).waitStressSidecar(ctx, "owned-stress", cntr, task, exited, output, errs)
			err := requireStressError(t, output, errs, time.Second)
			require.ErrorContains(t, err, "exited with code 137")
			if !errdefs.IsNotFound(killErr) {
				require.ErrorIs(t, err, killErr)
			}
		})
	}
}

func TestStressExecCanceledStatus(t *testing.T) {
	grpcCanceled := grpcstatus.Error(codes.Canceled, "SDK wait canceled")
	grpcDeadline := grpcstatus.Error(codes.DeadlineExceeded, "SDK deadline expired")
	workloadErr := errors.New("workload failed")
	for _, tt := range []struct {
		name     string
		err      error
		canceled error
		want     bool
	}{
		{name: "context", err: context.Canceled, canceled: context.Canceled, want: true},
		{name: "grpc canceled", err: grpcCanceled, canceled: context.Canceled, want: true},
		{name: "wrapped grpc", err: fmt.Errorf("exec: %w", grpcCanceled), canceled: context.Canceled, want: true},
		{name: "grpc deadline", err: grpcDeadline, canceled: context.DeadlineExceeded, want: true},
		{name: "wrong status", err: grpcCanceled, canceled: context.DeadlineExceeded},
		{name: "permission error", err: grpcstatus.Error(codes.PermissionDenied, "denied"), canceled: context.Canceled},
		{name: "joined context and workload", err: errors.Join(context.Canceled, workloadErr), canceled: context.Canceled},
		{name: "joined grpc and workload", err: errors.Join(grpcCanceled, workloadErr), canceled: context.Canceled},
		{name: "wrapped joined", err: fmt.Errorf("exec: %w", errors.Join(grpcCanceled, workloadErr)), canceled: context.Canceled},
		{name: "no error", canceled: context.Canceled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stressExecCanceled(tt.err, tt.canceled))
		})
	}
}
