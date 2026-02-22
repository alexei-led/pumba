package containerd

import (
	"bytes"
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
// This allows running tc/iptables commands even when the target container
// does not have these tools installed.
func (c *containerdClient) sidecarExec(ctx context.Context, target *ctr.Container, sidecarImage string, pull bool, command string, argsList [][]string) error {
	ctx = c.nsCtx(ctx)

	// 1. Pull sidecar image if requested
	if pull {
		if err := c.pullImage(ctx, sidecarImage); err != nil {
			return fmt.Errorf("failed to pull sidecar image %s: %w", sidecarImage, err)
		}
	}

	// 2. Get target container's task PID for network namespace
	targetTask, err := c.getTask(ctx, target.ID())
	if err != nil {
		return fmt.Errorf("failed to get target task for sidecar: %w", err)
	}
	targetPID := targetTask.Pid()

	// 3. Get the sidecar image
	image, err := c.client.GetImage(ctx, sidecarImage)
	if err != nil {
		return fmt.Errorf("failed to get sidecar image %s: %w", sidecarImage, err)
	}

	// 4. Create sidecar container sharing target's network namespace
	sidecarID := fmt.Sprintf("pumba-sidecar-%d", time.Now().UnixNano())
	sidecarContainer, err := c.client.NewContainer(ctx, sidecarID,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(sidecarID+"-snapshot", image),
		containerd.WithNewSpec(
			oci.WithImageConfig(image),
			oci.WithProcessArgs("sleep", "infinity"),
			// Share the target container's network namespace
			oci.WithLinuxNamespace(specs.LinuxNamespace{
				Type: specs.NetworkNamespace,
				Path: fmt.Sprintf("/proc/%d/ns/net", targetPID),
			}),
			// Need NET_ADMIN for tc and iptables
			oci.WithCapabilities([]string{"CAP_NET_ADMIN", "CAP_NET_RAW"}),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create sidecar container: %w", err)
	}

	// Cleanup: always remove the sidecar container
	defer func() {
		if cleanupErr := c.cleanupSidecar(ctx, sidecarContainer, sidecarID); cleanupErr != nil {
			log.WithError(cleanupErr).Warn("failed to clean up sidecar container")
		}
	}()

	// 5. Start sidecar task
	task, err := sidecarContainer.NewTask(ctx, cio.NullIO)
	if err != nil {
		return fmt.Errorf("failed to create sidecar task: %w", err)
	}
	if err := task.Start(ctx); err != nil {
		return fmt.Errorf("failed to start sidecar task: %w", err)
	}

	// 6. Execute each command set in the sidecar
	for _, args := range argsList {
		cmdArgs := append([]string{command}, args...)
		execID := fmt.Sprintf("pumba-sc-exec-%d", time.Now().UnixNano())
		pspec := &specs.Process{
			Args: cmdArgs,
			Cwd:  "/",
			Env:  []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
			User: specs.User{UID: 0, GID: 0},
			Capabilities: &specs.LinuxCapabilities{
				Effective:   []string{"CAP_NET_ADMIN", "CAP_NET_RAW"},
				Bounding:    []string{"CAP_NET_ADMIN", "CAP_NET_RAW"},
				Permitted:   []string{"CAP_NET_ADMIN", "CAP_NET_RAW"},
				Inheritable: []string{"CAP_NET_ADMIN", "CAP_NET_RAW"},
			},
		}

		var stdout, stderr bytes.Buffer
		execProcess, err := task.Exec(ctx, execID, pspec, cio.NewCreator(
			cio.WithStreams(nil, &stdout, &stderr),
		))
		if err != nil {
			return fmt.Errorf("failed to exec %s in sidecar: %w", strings.Join(cmdArgs, " "), err)
		}

		exitCh, err := execProcess.Wait(ctx)
		if err != nil {
			_, _ = execProcess.Delete(ctx)
			return fmt.Errorf("failed to wait on sidecar exec: %w", err)
		}

		if err := execProcess.Start(ctx); err != nil {
			_, _ = execProcess.Delete(ctx)
			return fmt.Errorf("failed to start sidecar exec: %w", err)
		}

		status := <-exitCh
		code, _, err := status.Result()
		if err != nil {
			return fmt.Errorf("sidecar exec failed: %w", err)
		}
		if _, err := execProcess.Delete(ctx); err != nil {
			log.WithError(err).Warn("failed to delete sidecar exec process")
		}
		if code != 0 {
			return fmt.Errorf("sidecar exec '%s' exited with code %d: %s",
				strings.Join(cmdArgs, " "), code, stderr.String())
		}
		log.WithField("args", strings.Join(args, " ")).Debug("sidecar exec completed")
	}

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

// cleanupSidecar kills the task and removes the sidecar container and its snapshot.
func (c *containerdClient) cleanupSidecar(ctx context.Context, cntr containerd.Container, snapshotID string) error {
	// Kill and delete task
	task, err := cntr.Task(ctx, nil)
	if err == nil {
		waitCh, _ := task.Wait(ctx)
		_ = task.Kill(ctx, syscall.SIGKILL)
		select {
		case <-waitCh:
		case <-time.After(5 * time.Second):
		}
		_, _ = task.Delete(ctx)
	} else if !errdefs.IsNotFound(err) {
		log.WithError(err).Warn("failed to get sidecar task for cleanup")
	}

	// Delete container with snapshot cleanup
	if err := cntr.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to delete sidecar container: %w", err)
		}
	}
	return nil
}
