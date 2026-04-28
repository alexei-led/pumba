package containerd

import (
	"context"
	"fmt"
	"syscall"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"
)

// buildStressSpecOpts builds OCI spec options for the stress sidecar container.
// For inject-cgroup mode (uses cgroupPath): runs /cg-inject with host cgroupns and /sys/fs/cgroup bind mount.
// For default sidecar mode (uses cgroupParent + sidecarID): runs /stress-ng directly, placed in a child cgroup
// under the target's cgroup parent. The child path format depends on the cgroup driver (systemd vs cgroupfs).
func buildStressSpecOpts(image containerd.Image, stressors []string, cgroupPath, cgroupParent, sidecarID string, injectCgroup bool) []oci.SpecOpts {
	if injectCgroup {
		prefix := []string{"/cg-inject", "--cgroup-path", cgroupPath, "--", "/stress-ng"}
		args := make([]string, 0, len(prefix)+len(stressors))
		args = append(args, prefix...)
		args = append(args, stressors...)
		return []oci.SpecOpts{
			oci.WithImageConfig(image),
			oci.WithProcessArgs(args...),
			oci.WithLinuxNamespace(specs.LinuxNamespace{
				Type: specs.CgroupNamespace,
				Path: "/proc/1/ns/cgroup",
			}),
			oci.WithMounts([]specs.Mount{
				{
					Type:        "bind",
					Source:      "/sys/fs/cgroup",
					Destination: "/sys/fs/cgroup",
					Options:     []string{"bind", "rw", "nosuid", "nodev", "noexec"},
				},
			}),
		}
	}
	args := make([]string, 0, len(stressors)+1)
	args = append(args, "/stress-ng")
	args = append(args, stressors...)
	return []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithProcessArgs(args...),
		oci.WithCgroup(cgroupChildPath(cgroupParent, sidecarID)),
	}
}

// createStressSidecar resolves the target cgroup path, optionally pulls the image, and creates and starts the stress sidecar container.
func (c *containerdClient) createStressSidecar(
	ctx context.Context,
	target *ctr.Container,
	sidecarImage string,
	stressors []string,
	injectCgroup bool,
	pull bool,
) (string, containerd.Container, containerd.Task, <-chan containerd.ExitStatus, error) {
	targetTask, err := c.getTask(ctx, target.ID())
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("failed to get target task for stress sidecar: %w", err)
	}
	targetPID := targetTask.Pid()
	if targetPID == 0 {
		return "", nil, nil, nil, fmt.Errorf("target task for %s has PID 0 (not running)", target.ID())
	}

	cgroupPath, cgroupParent, err := resolveCgroupPath(targetPID)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("failed to resolve cgroup for stress sidecar: %w", err)
	}

	if pull {
		if err := c.pullImage(ctx, sidecarImage); err != nil {
			return "", nil, nil, nil, fmt.Errorf("failed to pull stress image: %w", err)
		}
	}
	log.WithFields(log.Fields{
		"target":        target.ID(),
		"pid":           targetPID,
		"cgroup-path":   cgroupPath,
		"cgroup-parent": cgroupParent,
		"inject-cgroup": injectCgroup,
	}).Debug("resolved target cgroup for stress sidecar")

	image, err := c.client.GetImage(ctx, sidecarImage)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("failed to get stress image %s: %w", sidecarImage, err)
	}

	sidecarID := fmt.Sprintf("pumba-stress-%d", execCounter.Add(1))
	specOpts := buildStressSpecOpts(image, stressors, cgroupPath, cgroupParent, sidecarID, injectCgroup)

	sidecarContainer, err := c.client.NewContainer(ctx, sidecarID,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(sidecarID+"-snapshot", image),
		containerd.WithNewSpec(specOpts...),
		containerd.WithContainerLabels(map[string]string{"com.gaiaadm.pumba.skip": "true"}),
	)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("failed to create stress sidecar: %w", err)
	}

	task, waitCh, err := c.startSidecarTask(ctx, sidecarContainer)
	if err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarCleanupTimeout)
		c.deleteContainer(c.nsCtx(cleanupCtx), sidecarContainer)
		cancel()
		return "", nil, nil, nil, err
	}

	return sidecarID, sidecarContainer, task, waitCh, nil
}

// startSidecarTask creates, registers wait, and starts a task on the given container.
func (c *containerdClient) startSidecarTask(ctx context.Context, cntr containerd.Container) (containerd.Task, <-chan containerd.ExitStatus, error) {
	task, err := cntr.NewTask(ctx, cio.NullIO)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stress task: %w", err)
	}

	waitCh, err := task.Wait(ctx)
	if err != nil {
		_, _ = task.Delete(context.WithoutCancel(ctx))
		return nil, nil, fmt.Errorf("failed to set up stress task wait: %w", err)
	}

	if err := task.Start(ctx); err != nil {
		_, _ = task.Delete(context.WithoutCancel(ctx))
		return nil, nil, fmt.Errorf("failed to start stress task: %w", err)
	}

	return task, waitCh, nil
}

// waitStressSidecar waits for the stress task to exit, performs cleanup, and reports the result.
func (c *containerdClient) waitStressSidecar(
	ctx context.Context,
	sidecarID string,
	sidecarContainer containerd.Container,
	task containerd.Task,
	waitCh <-chan containerd.ExitStatus,
	outCh chan<- string,
	errCh chan<- error,
) {
	defer close(outCh)
	defer close(errCh)

	var status containerd.ExitStatus
	var ok, killTimedOut bool
	select {
	case status, ok = <-waitCh:
	case <-ctx.Done():
		if killErr := task.Kill(context.WithoutCancel(ctx), syscall.SIGKILL); killErr != nil && !errdefs.IsNotFound(killErr) {
			log.WithError(killErr).WithField("id", sidecarID).Warn("failed to kill stress task")
		}
		killTimer := time.NewTimer(sidecarKillTimeout)
		defer killTimer.Stop()
		select {
		case status, ok = <-waitCh:
		case <-killTimer.C:
			log.WithField("id", sidecarID).Warn("timed out waiting for stress task after kill")
			killTimedOut = true
		}
	}

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarCleanupTimeout)
	cleanupCtx = c.nsCtx(cleanupCtx)
	if _, delErr := task.Delete(cleanupCtx); delErr != nil && !errdefs.IsNotFound(delErr) {
		log.WithError(delErr).WithField("id", sidecarID).Warn("failed to delete stress task")
	}
	c.deleteContainer(cleanupCtx, sidecarContainer)
	cancel()

	if killTimedOut {
		errCh <- fmt.Errorf("stress sidecar %s: timed out waiting for task to exit after SIGKILL", sidecarID)
		return
	}
	if !ok {
		errCh <- fmt.Errorf("stress sidecar %s: wait channel closed unexpectedly", sidecarID)
		return
	}
	code, _, exitErr := status.Result()
	if exitErr != nil {
		errCh <- fmt.Errorf("stress sidecar error: %w", exitErr)
		return
	}
	if code != 0 {
		errCh <- fmt.Errorf("stress sidecar exited with code %d", code)
		return
	}
	outCh <- sidecarID
}

// deleteContainer deletes cntr and its snapshot.
// The caller must provide a context with a timeout and the containerd namespace already set.
func (c *containerdClient) deleteContainer(ctx context.Context, cntr containerd.Container) {
	if err := cntr.Delete(ctx, containerd.WithSnapshotCleanup); err != nil && !errdefs.IsNotFound(err) {
		log.WithError(err).Warn("failed to clean up sidecar container")
	}
}
