package containerd

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	containerd "github.com/containerd/containerd/v2/client"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// StressContainer runs stress-ng to stress a container.
// Mode selection:
//   - Sidecar.Image == "": direct exec inside the target container
//   - Sidecar.Image != "" && !InjectCgroup: sidecar with /stress-ng in target's cgroup parent
//   - Sidecar.Image != "" && InjectCgroup: sidecar with /cg-inject injecting into target's cgroup
func (c *containerdClient) StressContainer(ctx context.Context, req *ctr.StressRequest) (*ctr.StressResult, error) {
	log.WithFields(log.Fields{
		"id":            req.Container.ID(),
		"image":         req.Sidecar.Image,
		"inject-cgroup": req.InjectCgroup,
	}).Debug("stress on containerd container")
	if req.DryRun {
		return &ctr.StressResult{}, nil
	}
	if req.Sidecar.Image != "" {
		id, outCh, errCh, err := c.stressSidecar(ctx, req)
		if err != nil {
			return nil, err
		}
		return &ctr.StressResult{SidecarID: id, Output: outCh, Errors: errCh}, nil
	}
	id, outCh, errCh := c.stressDirectExec(ctx, req.Container, req.Stressors, req.Duration)
	return &ctr.StressResult{SidecarID: id, Output: outCh, Errors: errCh}, nil
}

// stressDirectExec runs stress-ng directly inside the target container via exec.
func (c *containerdClient) stressDirectExec(ctx context.Context, container *ctr.Container,
	stressors []string, duration time.Duration) (string, <-chan string, <-chan error) {
	errCh := make(chan error, 1)
	outCh := make(chan string, 1)
	go func() {
		defer close(errCh)
		defer close(outCh)
		secs := max(1, int(math.Ceil(duration.Seconds())))
		timeoutArgs := []string{"--timeout", fmt.Sprintf("%ds", secs)}
		args := make([]string, 0, len(timeoutArgs)+len(stressors))
		args = append(args, timeoutArgs...)
		args = append(args, stressors...)
		if _, _, err := c.execContainerCommand(c.nsCtx(ctx), container.ID(), "stress-ng", args, execStressTask); err != nil {
			errCh <- err
			return
		}
		outCh <- container.ID()
	}()
	return "", outCh, errCh
}

// stressSidecar creates a long-lived sidecar container running stress-ng (or cg-inject)
// as its main process. Returns the sidecar ID and output/error channels. A goroutine waits
// for the task to exit and performs full cleanup (task delete + container/snapshot removal).
func (c *containerdClient) stressSidecar(ctx context.Context, req *ctr.StressRequest) (string, <-chan string, <-chan error, error) {
	ctx = c.nsCtx(ctx)

	waitCtx, waitCancel := context.WithCancel(context.WithoutCancel(ctx))
	sidecarID, sidecarContainer, task, waitCh, err := c.createStressSidecar(ctx, waitCtx, req)
	if err != nil {
		waitCancel()
		return "", nil, nil, err
	}

	outCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		defer waitCancel()
		c.waitStressSidecar(ctx, sidecarID, sidecarContainer, task, waitCh, outCh, errCh)
	}()

	return sidecarID, outCh, errCh, nil
}

// Stress owns only this exec process, never the target task. User exec keeps
// its existing non-terminating cancellation behavior.
func execStressTask(ctx context.Context, task containerd.Task, pspec *specs.Process, execID, description string) (string, string, error) {
	out, errOut, err := runTaskExec(ctx, task, pspec, execID, description, terminateStressExec)
	if errors.Is(err, errExecCleanup) {
		return out, errOut, &ctr.StressCleanupError{Err: err}
	}
	if ctx.Err() != nil && stressExecCanceled(err, ctx.Err()) {
		return out, errOut, nil
	}
	return out, errOut, err
}

// Joined cleanup/workload failures are never an expected cancellation alone.
func stressExecCanceled(err, canceled error) bool {
	for errors.Unwrap(err) != nil {
		err = errors.Unwrap(err)
	}
	if _, joined := err.(interface{ Unwrap() []error }); joined {
		return false
	}
	if errors.Is(err, canceled) {
		return true
	}
	return (errors.Is(canceled, context.Canceled) && grpcstatus.Code(err) == codes.Canceled) ||
		(errors.Is(canceled, context.DeadlineExceeded) && grpcstatus.Code(err) == codes.DeadlineExceeded)
}
