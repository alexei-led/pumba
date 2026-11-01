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

const killedExitCode = 128 + uint32(syscall.SIGKILL)

type execCleanupResult struct {
	exit   *containerd.ExitStatus
	killed bool
	err    error
}

// Cleanup owns only this exec, never the target task. Delete's terminal status
// remains authoritative when cancellation made the original wait unavailable.
func cleanupTaskExec(ctx context.Context, process containerd.Process, terminate bool) execCleanupResult {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarCleanupTimeout)
	defer cancel()
	var result execCleanupResult
	if terminate {
		result = terminateOwnedExec(cleanupCtx, process)
		if result.err != nil {
			result.err = fmt.Errorf("failed to terminate owned exec; subprocesses may remain: %w", result.err)
		}
	}
	exit, deleteErr := process.Delete(cleanupCtx)
	if exit != nil {
		result.exit = exit
	}
	if errdefs.IsNotFound(deleteErr) {
		deleteErr = nil
	}
	if deleteErr != nil {
		deleteErr = fmt.Errorf("failed to delete exec process: %w", deleteErr)
	}
	result.err = errors.Join(result.err, deleteErr)
	return result
}

func terminateOwnedExec(ctx context.Context, process containerd.Process) execCleanupResult {
	waitCh, waitErr := process.Wait(ctx)
	if errdefs.IsNotFound(waitErr) {
		return execCleanupResult{}
	}
	killErr := process.Kill(ctx, syscall.SIGKILL)
	if errdefs.IsNotFound(killErr) {
		return execCleanupResult{err: waitErr}
	}
	result := execCleanupResult{killed: killErr == nil}
	if waitErr != nil || killErr != nil {
		result.err = errors.Join(waitErr, killErr)
		return result
	}
	select {
	case status, ok := <-waitCh:
		if !ok {
			result.err = errors.New("owned exec wait channel closed without exit status")
		} else {
			result.exit = &status
			_, _, result.err = status.Result()
		}
	case <-ctx.Done():
		result.err = fmt.Errorf("waiting for owned exec after SIGKILL: %w", ctx.Err())
	}
	return result
}

func canceledExecResultError(description string, cleanup execCleanupResult) error {
	if cleanup.exit == nil {
		return fmt.Errorf("%s: %w: terminal status unavailable after cancellation", description, errExecCleanup)
	}
	code, _, err := cleanup.exit.Result()
	if err != nil {
		return fmt.Errorf("%s: %w: failed to observe terminal status: %w", description, errExecCleanup, err)
	}
	if code == 0 || (cleanup.killed && code == killedExitCode) {
		return nil
	}
	return fmt.Errorf("%s exited with code %d", description, code)
}

func execStatusKnown(status containerd.ExitStatus, ok bool) bool {
	if !ok {
		return false
	}
	_, _, err := status.Result()
	return err == nil
}
