package docker

import (
	"context"
	"fmt"
	"io"
	"strings"

	ctr "github.com/alexei-led/pumba/pkg/container"
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

	output, err := io.ReadAll(attachRes.Reader)
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

// runExecAttached starts a pre-created exec by attaching to it and draining
// stdout/stderr until the exec completes. Podman's Docker-compat API rejects
// ContainerExecStart with empty ExecStartOptions ("must provide at least one
// stream to attach to"); Docker accepts it. ContainerExecAttach works on both.
func (client dockerClient) runExecAttached(ctx context.Context, execID string) error {
	resp, err := client.containerAPI.ContainerExecAttach(ctx, execID, ctypes.ExecAttachOptions{})
	if err != nil {
		return err
	}
	defer resp.Close()
	if _, err := io.ReadAll(resp.Reader); err != nil {
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
