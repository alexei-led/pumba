package containerd

import (
	"context"
	"errors"
	"syscall"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/errdefs"
)

// stress-ng must get a chance to reap its workers. Killing its exec parent
// immediately can orphan workers in the target's still-live PID namespace.
func terminateStressExec(ctx context.Context, process containerd.Process) execCleanupResult {
	waitCh, waitErr := process.Wait(ctx)
	if errdefs.IsNotFound(waitErr) {
		return execCleanupResult{}
	}
	termErr := process.Kill(ctx, syscall.SIGTERM)
	if errdefs.IsNotFound(termErr) {
		return execCleanupResult{err: waitErr}
	}
	if termErr == nil {
		timer := time.NewTimer(sidecarKillTimeout)
		defer timer.Stop()
		select {
		case status, ok := <-waitCh:
			if ok {
				_, _, statusErr := status.Result()
				if statusErr == nil {
					return execCleanupResult{exit: &status, expectedExitCode: signalExitBase + uint32(syscall.SIGTERM), err: waitErr}
				}
				waitErr = errors.Join(waitErr, statusErr)
			} else {
				waitErr = errors.Join(waitErr, errors.New("stress exec wait channel closed without exit status"))
			}
		case <-timer.C:
		case <-ctx.Done():
		}
	}
	result := terminateOwnedExec(ctx, process)
	result.err = errors.Join(waitErr, termErr, result.err)
	return result
}
