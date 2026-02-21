package containerd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/errdefs"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"
)

// signalMap maps signal names to syscall signals.
var signalMap = map[string]syscall.Signal{
	"SIGTERM": syscall.SIGTERM,
	"SIGKILL": syscall.SIGKILL,
	"SIGINT":  syscall.SIGINT,
	"SIGHUP":  syscall.SIGHUP,
	"SIGUSR1": syscall.SIGUSR1,
	"SIGUSR2": syscall.SIGUSR2,
	"SIGSTOP": syscall.SIGSTOP,
}

func parseSignal(signal string) syscall.Signal {
	signal = strings.ToUpper(signal)
	if !strings.HasPrefix(signal, "SIG") {
		signal = "SIG" + signal
	}
	if sig, ok := signalMap[signal]; ok {
		return sig
	}
	return syscall.SIGKILL
}

func (c *containerdClient) stopTask(ctx context.Context, containerID string, timeout time.Duration) error {
	cntr, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container %s: %w", containerID, err)
	}
	task, err := cntr.Task(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get task for %s: %w", containerID, err)
	}
	waitCh, err := task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait on task %s: %w", containerID, err)
	}
	// Send SIGTERM first
	if err := task.Kill(ctx, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to %s: %w", containerID, err)
	}
	select {
	case <-waitCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context canceled waiting for SIGTERM on %s: %w", containerID, ctx.Err())
	case <-time.After(timeout):
		log.WithField("id", containerID).Debug("SIGTERM timeout, sending SIGKILL")
		if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to send SIGKILL to %s: %w", containerID, err)
		}
		select {
		case <-waitCh:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("context canceled waiting for SIGKILL on %s: %w", containerID, ctx.Err())
		}
	}
}

func (c *containerdClient) killTask(ctx context.Context, containerID, signal string) error {
	cntr, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container %s: %w", containerID, err)
	}
	task, err := cntr.Task(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get task for %s: %w", containerID, err)
	}
	sig := parseSignal(signal)
	return task.Kill(ctx, sig)
}

func (c *containerdClient) startTask(ctx context.Context, containerID string) error {
	cntr, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container %s: %w", containerID, err)
	}
	task, err := cntr.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to get task for %s: %w", containerID, err)
		}
		task, err = cntr.NewTask(ctx, cio.NewCreator(cio.WithStdio))
		if err != nil {
			return fmt.Errorf("failed to create task for %s: %w", containerID, err)
		}
	}
	return task.Start(ctx)
}

func (c *containerdClient) pauseTask(ctx context.Context, containerID string) error {
	cntr, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container %s: %w", containerID, err)
	}
	task, err := cntr.Task(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get task for %s: %w", containerID, err)
	}
	return task.Pause(ctx)
}

func (c *containerdClient) resumeTask(ctx context.Context, containerID string) error {
	cntr, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container %s: %w", containerID, err)
	}
	task, err := cntr.Task(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get task for %s: %w", containerID, err)
	}
	return task.Resume(ctx)
}

func (c *containerdClient) execInContainer(ctx context.Context, containerID, command string, args []string) error {
	cntr, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container %s: %w", containerID, err)
	}
	task, err := cntr.Task(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get task for %s: %w", containerID, err)
	}

	cmdArgs := append([]string{command}, args...)
	execID := fmt.Sprintf("pumba-exec-%d", time.Now().UnixNano())
	pspec := &specs.Process{
		Args: cmdArgs,
		Cwd:  "/",
		Env:  []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
	}

	var stdout, stderr bytes.Buffer
	execProcess, err := task.Exec(ctx, execID, pspec, cio.NewCreator(
		cio.WithStreams(nil, &stdout, &stderr),
	))
	if err != nil {
		return fmt.Errorf("failed to exec in %s: %w", containerID, err)
	}

	exitCh, err := execProcess.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait on exec in %s: %w", containerID, err)
	}

	if err := execProcess.Start(ctx); err != nil {
		_, _ = execProcess.Delete(ctx)
		return fmt.Errorf("failed to start exec in %s: %w", containerID, err)
	}

	status := <-exitCh
	code, _, err := status.Result()
	if err != nil {
		return fmt.Errorf("exec in %s failed: %w", containerID, err)
	}

	if _, err := execProcess.Delete(ctx); err != nil {
		log.WithError(err).WithField("id", containerID).Warn("failed to delete exec process")
	}

	if code != 0 {
		return fmt.Errorf("exec in %s exited with code %d: %s", containerID, code, stderr.String())
	}
	return nil
}
