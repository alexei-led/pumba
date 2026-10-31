package containerd

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/errdefs"
)

const stressKilledExitCode = 128 + uint32(syscall.SIGKILL)

type stressTaskExit struct {
	status               containerd.ExitStatus
	ok, killed, timedOut bool
	cleanupErr           error
}

// The wait subscription survives workload cancellation. Prefer an exit already
// delivered before killing, so natural failures cannot be mistaken for our stop.
func waitStressTask(ctx context.Context, task containerd.Task, waitCh <-chan containerd.ExitStatus) stressTaskExit {
	var exit stressTaskExit
	select {
	case exit.status, exit.ok = <-waitCh:
		return exit
	default:
	}
	select {
	case exit.status, exit.ok = <-waitCh:
		return exit
	case <-ctx.Done():
	}
	select {
	case exit.status, exit.ok = <-waitCh:
		return exit
	default:
	}
	killCtx, killCancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarKillTimeout)
	exit.cleanupErr = task.Kill(killCtx, syscall.SIGKILL)
	exit.killed = exit.cleanupErr == nil
	killCancel()
	if errdefs.IsNotFound(exit.cleanupErr) {
		exit.cleanupErr = nil
	}
	timer := time.NewTimer(sidecarKillTimeout)
	defer timer.Stop()
	select {
	case exit.status, exit.ok = <-waitCh:
	case <-timer.C:
		exit.timedOut = true
		exit.cleanupErr = errors.Join(exit.cleanupErr, fmt.Errorf("waiting for stress task after SIGKILL: %w", context.DeadlineExceeded))
	}
	return exit
}
