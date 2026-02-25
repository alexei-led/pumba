package containerd

import (
	"context"
	"fmt"
	"strings"
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

// sidecarExec creates a short-lived sidecar container that shares the target
// container's network namespace and runs the given command+args inside it.
func (c *containerdClient) sidecarExec(ctx context.Context, target *ctr.Container, sidecarImage string, pull bool, command string, argsList [][]string) error {
	ctx = c.nsCtx(ctx)

	if pull {
		if err := c.pullImage(ctx, sidecarImage); err != nil {
			return fmt.Errorf("failed to pull sidecar image %s: %w", sidecarImage, err)
		}
	}

	targetTask, err := c.getTask(ctx, target.ID())
	if err != nil {
		return fmt.Errorf("failed to get target task for sidecar: %w", err)
	}
	targetPID := targetTask.Pid()
	if targetPID == 0 {
		return fmt.Errorf("target task for %s has PID 0 (not running)", target.ID())
	}

	image, err := c.client.GetImage(ctx, sidecarImage)
	if err != nil {
		return fmt.Errorf("failed to get sidecar image %s: %w", sidecarImage, err)
	}

	sidecarID := fmt.Sprintf("pumba-sidecar-%d", execCounter.Add(1))
	sidecarContainer, err := c.client.NewContainer(ctx, sidecarID,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(sidecarID+"-snapshot", image),
		containerd.WithNewSpec(
			oci.WithImageConfig(image),
			oci.WithProcessArgs("sleep", "infinity"),
			oci.WithLinuxNamespace(specs.LinuxNamespace{
				Type: specs.NetworkNamespace,
				Path: fmt.Sprintf("/proc/%d/ns/net", targetPID),
			}),
			oci.WithCapabilities(networkCapabilities),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create sidecar container: %w", err)
	}

	// Use context.WithoutCancel so cleanup succeeds even if the parent ctx is canceled.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarCleanupTimeout)
		defer cancel()
		cleanupCtx = c.nsCtx(cleanupCtx)
		if cleanupErr := c.cleanupSidecar(cleanupCtx, sidecarContainer); cleanupErr != nil {
			log.WithError(cleanupErr).Warn("failed to clean up sidecar container")
		}
	}()

	task, err := sidecarContainer.NewTask(ctx, cio.NullIO)
	if err != nil {
		return fmt.Errorf("failed to create sidecar task: %w", err)
	}
	if err := task.Start(ctx); err != nil {
		return fmt.Errorf("failed to start sidecar task: %w", err)
	}

	for _, args := range argsList {
		if err := c.runSidecarCmd(ctx, task, command, args); err != nil {
			return err
		}
	}

	return nil
}

// runSidecarCmd executes a single command inside a running sidecar task.
func (c *containerdClient) runSidecarCmd(ctx context.Context, task containerd.Task, command string, args []string) error {
	cmdArgs := make([]string, 0, 1+len(args))
	cmdArgs = append(cmdArgs, command)
	cmdArgs = append(cmdArgs, args...)
	execID := fmt.Sprintf("pumba-sc-exec-%d", execCounter.Add(1))
	pspec := &specs.Process{
		Args: cmdArgs,
		Cwd:  "/",
		Env:  []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		User: specs.User{UID: 0, GID: 0},
		Capabilities: &specs.LinuxCapabilities{
			Effective:   networkCapabilities,
			Bounding:    networkCapabilities,
			Permitted:   networkCapabilities,
			Inheritable: networkCapabilities,
		},
	}

	if err := execTask(ctx, task, pspec, execID, fmt.Sprintf("sidecar exec '%s'", strings.Join(cmdArgs, " "))); err != nil {
		return err
	}
	log.WithField("args", strings.Join(args, " ")).Debug("sidecar exec completed")
	return nil
}

// pullImage pulls an image via containerd.
func (c *containerdClient) pullImage(ctx context.Context, ref string) error {
	log.WithField("image", ref).Debug("pulling image via containerd")
	_, err := c.client.Pull(ctx, ref, containerd.WithPullUnpack)
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", ref, err)
	}
	return nil
}

const (
	sidecarCleanupTimeout = 30 * time.Second
	sidecarKillTimeout    = 5 * time.Second
)

// networkCapabilities are the Linux capabilities required for network manipulation.
var networkCapabilities = []string{"CAP_NET_ADMIN", "CAP_NET_RAW"}

// cleanupSidecar kills the task and removes the sidecar container and its snapshot.
func (c *containerdClient) cleanupSidecar(ctx context.Context, cntr containerd.Container) error {
	task, err := cntr.Task(ctx, nil)
	if err == nil {
		waitCh, waitErr := task.Wait(ctx)
		_ = task.Kill(ctx, syscall.SIGKILL)
		if waitErr == nil {
			killTimer := time.NewTimer(sidecarKillTimeout)
			defer killTimer.Stop()
			select {
			case <-waitCh:
			case <-killTimer.C:
			}
		}
		_, _ = task.Delete(ctx)
	} else if !errdefs.IsNotFound(err) {
		log.WithError(err).Warn("failed to get sidecar task for cleanup")
	}

	if err := cntr.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to delete sidecar container: %w", err)
		}
	}
	return nil
}
