package containerd

import (
	"context"
	"fmt"
	"math"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
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
		id, outCh, errCh, err := c.stressSidecar(ctx, req.Container, req.Sidecar.Image, req.Stressors, req.InjectCgroup, req.Sidecar.Pull)
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
		if err := c.execInContainer(c.nsCtx(ctx), container.ID(), "stress-ng", args); err != nil {
			errCh <- err
			return
		}
		outCh <- container.ID()
	}()
	return container.ID(), outCh, errCh
}

// stressSidecar creates a long-lived sidecar container running stress-ng (or cg-inject)
// as its main process. Returns the sidecar ID and output/error channels. A goroutine waits
// for the task to exit and performs full cleanup (task delete + container/snapshot removal).
func (c *containerdClient) stressSidecar(
	ctx context.Context,
	target *ctr.Container,
	sidecarImage string,
	stressors []string,
	injectCgroup bool,
	pull bool,
) (string, <-chan string, <-chan error, error) {
	ctx = c.nsCtx(ctx)

	sidecarID, sidecarContainer, task, waitCh, err := c.createStressSidecar(ctx, target, sidecarImage, stressors, injectCgroup, pull)
	if err != nil {
		return "", nil, nil, err
	}

	outCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go c.waitStressSidecar(ctx, sidecarID, sidecarContainer, task, waitCh, outCh, errCh)

	return sidecarID, outCh, errCh, nil
}
