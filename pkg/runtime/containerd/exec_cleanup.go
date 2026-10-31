package containerd

import (
	"context"
	"errors"
	"fmt"
	"syscall"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/errdefs"
)

var errExecCleanup = errors.New("exec cleanup failed")

// cleanupTaskExec only terminates an exec when a fault command exceeds its
// internal deadline. It never signals the target task or unrelated execs.
func cleanupTaskExec(ctx context.Context, process containerd.Process, terminate bool) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarCleanupTimeout)
	defer cancel()
	var terminationErr error
	if terminate {
		if err := terminateFaultExec(cleanupCtx, process); err != nil {
			terminationErr = fmt.Errorf("failed to terminate fault exec; fault subprocesses may remain: %w", err)
		}
	}
	_, deleteErr := process.Delete(cleanupCtx)
	if errdefs.IsNotFound(deleteErr) {
		deleteErr = nil
	}
	if deleteErr != nil {
		deleteErr = fmt.Errorf("failed to delete exec process: %w", deleteErr)
	}
	return errors.Join(terminationErr, deleteErr)
}

func terminateFaultExec(ctx context.Context, process containerd.Process) error {
	waitCh, waitErr := process.Wait(ctx)
	if errdefs.IsNotFound(waitErr) {
		return nil
	}
	killErr := process.Kill(ctx, syscall.SIGKILL)
	if errdefs.IsNotFound(killErr) {
		return nil
	}
	if waitErr != nil || killErr != nil {
		return errors.Join(waitErr, killErr)
	}
	select {
	case status, ok := <-waitCh:
		if !ok {
			return errors.New("fault exec wait channel closed without exit status")
		}
		_, _, err := status.Result()
		return err
	case <-ctx.Done():
		return fmt.Errorf("waiting for fault exec after SIGKILL: %w", ctx.Err())
	}
}
