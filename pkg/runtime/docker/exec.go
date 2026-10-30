package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
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
	// --max-duration and SIGTERM must interrupt it (unlike runExec).
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
// cancellation (see runExec). tc/iptables scripts finish in well under
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
	log.WithField("command", execCmd).Debugf("checking if command exists")
	if _, _, err := client.runExec(ctx, c.ID(), []string{"which", execCmd}, false); err != nil {
		return fmt.Errorf("command '%s' not found inside the %s container: %w", execCmd, c.ID(), err)
	}

	// if command found execute it
	log.WithField("command", execCmd).Debug("command found: continue execution")
	if _, _, err := client.runExec(ctx, c.ID(), append([]string{execCmd}, execArgs...), privileged); err != nil {
		return fmt.Errorf("command '%s' failed in %s container: %w", execCmd, c.ID(), err)
	}
	return nil
}

// runExec runs argv as an exec in containerID and returns its stdout and
// stderr. A non-zero exit is an error that carries stderr, so callers can name
// the cause (e.g. a missing iptables extension). It attaches with
// ContainerExecAttach, which Podman's Docker-compat API accepts where
// ContainerExecStart with empty options is rejected.
//
// It ignores the caller's cancellation, bounded by faultExecTimeout: its
// callers install, remove or inspect faults (or probe for a command), and a tc
// script killed midway leaves a partial topology the verified stop script
// refuses to remove.
func (client dockerClient) runExec(ctx context.Context, containerID string, argv []string, privileged bool) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), faultExecTimeout)
	defer cancel()
	created, err := client.containerAPI.ContainerExecCreate(ctx, containerID, ctypes.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Privileged:   privileged,
		Cmd:          argv,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create exec %s: %w", strings.Join(argv, " "), err)
	}
	resp, err := client.containerAPI.ContainerExecAttach(ctx, created.ID, ctypes.ExecAttachOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to start exec %s: %w", strings.Join(argv, " "), err)
	}
	raw, err := readAttached(ctx, resp)
	resp.Close()
	if err != nil {
		return "", "", fmt.Errorf("failed to read exec %s output: %w", strings.Join(argv, " "), err)
	}
	insp, err := client.containerAPI.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return "", "", fmt.Errorf("failed to inspect exec %s: %w", strings.Join(argv, " "), err)
	}
	// Without a TTY the attach stream multiplexes stdout and stderr.
	var outBuf, errBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&outBuf, &errBuf, bytes.NewReader(raw)); err != nil {
		return "", "", fmt.Errorf("failed to demultiplex exec %s output: %w", strings.Join(argv, " "), err)
	}
	if insp.ExitCode != 0 {
		return "", "", fmt.Errorf("%s failed with exit code %d: %s", strings.Join(argv, " "), insp.ExitCode, strings.TrimSpace(errBuf.String()))
	}
	return outBuf.String(), errBuf.String(), nil
}
