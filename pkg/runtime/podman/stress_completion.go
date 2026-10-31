package podman

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
)

const sidecarRemoveTimeout = 10 * time.Second

func removeStressSidecar(ctx context.Context, api apiBackend, id string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarRemoveTimeout)
	defer cancel()
	if err := api.ContainerRemove(cleanupCtx, id, ctypes.RemoveOptions{Force: true}); err != nil && !errdefs.IsNotFound(err) {
		return &ctr.StressCleanupError{Err: err}
	}
	return nil
}

// The attachment can reach EOF before the process exits. Wait for actual exit
// before removing the sidecar, and close both result channels only after removal.
func drainStressOutput(ctx context.Context, api apiBackend, id string, attach types.HijackedResponse, output chan<- string, outerr chan<- error) {
	defer close(output)
	defer close(outerr)
	defer attach.Close()
	stopClose := context.AfterFunc(ctx, attach.Close)
	defer stopClose()
	exited, waitErrors := api.ContainerWait(ctx, id, ctypes.WaitConditionNotRunning)
	var buf bytes.Buffer
	_, readErr := io.Copy(&buf, attach.Reader)
	if ctx.Err() != nil && stressInterruptedRead(ctx, readErr) {
		readErr = nil // only the read interrupted by our attachment close is expected
	}
	var exitErr error
	if readErr == nil {
		exitErr = waitStressExit(ctx, exited, waitErrors)
	}
	err := errors.Join(readErr, exitErr, removeStressSidecar(ctx, api, id))
	if err != nil {
		outerr <- err
	} else {
		output <- buf.String()
	}
}

func waitStressExit(ctx context.Context, exited <-chan ctypes.WaitResponse, waitErrors <-chan error) error {
	// Prefer an already-observed exit/failure to simultaneous cancellation.
	select {
	case status, ok := <-exited:
		return stressExitError(status, ok)
	case err := <-waitErrors:
		return stressWaitError(ctx, err, exited)
	default:
	}
	select {
	case status, ok := <-exited:
		return stressExitError(status, ok)
	case err := <-waitErrors:
		return stressWaitError(ctx, err, exited)
	case <-ctx.Done():
		// Recheck so a genuine failure already delivered is never erased.
		select {
		case status, ok := <-exited:
			return stressExitError(status, ok)
		default:
		}
		select {
		case err := <-waitErrors:
			return stressWaitError(ctx, err, exited)
		default:
			return nil
		}
	}
}

func stressWaitError(ctx context.Context, err error, exited <-chan ctypes.WaitResponse) error {
	if ctx.Err() != nil && errors.Is(stressErrorCause(err), ctx.Err()) {
		select {
		case status, ok := <-exited:
			return stressExitError(status, ok)
		default:
			return nil
		}
	}
	if err == nil {
		return errors.New("stress-ng wait channel closed without exit status")
	}
	return fmt.Errorf("failed to wait for stress-ng container: %w", err)
}

func stressExitError(status ctypes.WaitResponse, ok bool) error {
	if !ok {
		return errors.New("stress-ng wait channel closed without exit status")
	}
	if status.Error != nil {
		return fmt.Errorf("stress-ng wait failed: %s", status.Error.Message)
	}
	if status.StatusCode != 0 {
		return fmt.Errorf("stress-ng exited with code %d", status.StatusCode)
	}
	return nil
}

// Follow only a single wrapped cause. A joined error may also carry a genuine
// workload failure and must not be suppressed merely because cancellation matches.
func stressErrorCause(err error) error {
	for errors.Unwrap(err) != nil {
		err = errors.Unwrap(err)
	}
	if _, joined := err.(interface{ Unwrap() []error }); joined {
		return nil
	}
	return err
}

func stressInterruptedRead(ctx context.Context, err error) bool {
	cause := stressErrorCause(err)
	return errors.Is(cause, net.ErrClosed) || errors.Is(cause, io.ErrClosedPipe) || errors.Is(cause, ctx.Err())
}
