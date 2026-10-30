package docker

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	log "github.com/sirupsen/logrus"
)

// ExecContainer executes a command in a container
func (client dockerClient) ExecContainer(ctx context.Context, c *ctr.Container, command string, args []string, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"command": command,
		"dryrun":  dryrun,
	}).Info("exec container")
	if dryrun {
		return nil
	}
	createRes, err := client.containerAPI.ContainerExecCreate(
		ctx, c.ID(), ctypes.ExecOptions{
			User:         "root",
			AttachStdout: true,
			AttachStderr: true,
			Cmd:          append([]string{command}, args...),
		},
	)
	if err != nil {
		return fmt.Errorf("exec create failed: %w", err)
	}

	attachRes, err := client.containerAPI.ContainerExecAttach(
		ctx, createRes.ID, ctypes.ExecAttachOptions{},
	)
	if err != nil {
		return fmt.Errorf("exec attach failed: %w", err)
	}
	defer attachRes.Close()

	// User-facing `pumba exec` runs arbitrary, possibly endless commands, so
	// --max-duration and SIGTERM must interrupt it (unlike runExecAttached).
	output, err := readAttached(ctx, attachRes)
	if err != nil {
		return fmt.Errorf("reading output from exec reader failed: %w", err)
	}
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"command": command,
		"args":    args,
		"dryrun":  dryrun,
	}).Info(string(output))

	res, err := client.containerAPI.ContainerExecInspect(ctx, createRes.ID)
	if err != nil {
		return fmt.Errorf("exec inspect failed: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("exec failed %s: exit code %d", command, res.ExitCode)
	}
	return nil
}

// faultExecTimeout bounds a fault-installing exec that ignores the caller's
// cancellation (see runExecAttached). tc/iptables scripts finish in well under
// a second; this only guards against a hung runtime.
const faultExecTimeout = 2 * time.Minute

// readAttached reads a hijacked attach stream to EOF. The SDK uses ctx only to
// dial, so a blocked read ignores cancellation; a watcher closes the
// connection when ctx is done, and the ctx error is returned instead of a
// truncated result.
func readAttached(ctx context.Context, resp types.HijackedResponse) ([]byte, error) {
	var interrupted atomic.Bool
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			interrupted.Store(true)
			resp.Close()
		case <-done:
		}
	}()
	out, err := io.ReadAll(resp.Reader)
	if interrupted.Load() {
		return out, context.Cause(ctx)
	}
	return out, err
}

// runExecAttached starts a pre-created exec by attaching to it and draining
// stdout/stderr until the exec completes. Podman's Docker-compat API rejects
// ContainerExecStart with empty ExecStartOptions ("must provide at least one
// stream to attach to"); Docker accepts it. ContainerExecAttach works on both.
//
// Every caller installs or removes a fault (tc/iptables scripts in a sidecar
// or, in exec mode, the target), so the exec must run to completion: closing
// the stream on cancellation makes the caller remove the sidecar (SIGKILL) or
// run the stop script while the start script is midway, leaving a partial
// topology the verified stop script refuses to remove. These execs are short,
// so attach and read ignore ctx cancellation, bounded by faultExecTimeout.
// Only the user-facing ExecContainer reads interruptibly.
func (client dockerClient) runExecAttached(ctx context.Context, execID string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), faultExecTimeout)
	defer cancel()
	resp, err := client.containerAPI.ContainerExecAttach(ctx, execID, ctypes.ExecAttachOptions{})
	if err != nil {
		return err
	}
	defer resp.Close()
	if _, err := readAttached(ctx, resp); err != nil {
		return fmt.Errorf("drain exec %s output: %w", execID, err)
	}
	return nil
}

// execute command on container
func (client dockerClient) execOnContainer(ctx context.Context, c *ctr.Container, execCmd string, execArgs []string, privileged bool) error {
	log.WithFields(log.Fields{
		"id":         c.ID(),
		"name":       c.Name(),
		"command":    execCmd,
		"args":       execArgs,
		"privileged": privileged,
	}).Debug("executing command in container")
	// trim all spaces from cmd
	execCmd = strings.ReplaceAll(execCmd, " ", "")

	// check if command exists inside target container
	checkExists := ctypes.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"which", execCmd},
	}
	exec, err := client.containerAPI.ContainerExecCreate(ctx, c.ID(), checkExists)
	if err != nil {
		return fmt.Errorf("failed to create exec configuration to check if command exists: %w", err)
	}
	log.WithField("command", execCmd).Debugf("checking if command exists")
	if err = client.runExecAttached(ctx, exec.ID); err != nil {
		return fmt.Errorf("failed to check if command exists in a container: %w", err)
	}
	checkInspect, err := client.containerAPI.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect check execution: %w", err)
	}
	if checkInspect.ExitCode != 0 {
		return fmt.Errorf("command '%s' not found inside the %s container", execCmd, c.ID())
	}

	// if command found execute it
	log.WithField("command", execCmd).Debug("command found: continue execution")

	// prepare exec config
	config := ctypes.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Privileged:   privileged,
		Cmd:          append([]string{execCmd}, execArgs...),
	}
	// execute the command
	exec, err = client.containerAPI.ContainerExecCreate(ctx, c.ID(), config)
	if err != nil {
		return fmt.Errorf("failed to create exec configuration for a command: %w", err)
	}
	log.Debugf("starting exec %s %s (%s)", execCmd, execArgs, exec.ID)
	if err = client.runExecAttached(ctx, exec.ID); err != nil {
		return fmt.Errorf("failed to start command execution: %w", err)
	}
	exitInspect, err := client.containerAPI.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect command execution: %w", err)
	}
	if exitInspect.ExitCode != 0 {
		return fmt.Errorf("command '%s' failed in %s container; run it in manually to debug", execCmd, c.ID())
	}
	return nil
}
