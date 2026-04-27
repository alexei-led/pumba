package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	cerrdefs "github.com/containerd/errdefs"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

// sidecarRemoveTimeout bounds how long pumba will wait for ContainerRemove
// to reap an ephemeral tc/iptables sidecar after the caller's ctx cancels
// (e.g. SIGTERM). Podman's force-remove can take a few seconds on slow VMs.
const (
	sidecarRemoveTimeout  = 15 * time.Second
	sidecarInspectTimeout = 2 * time.Second
)

// removeSidecar force-removes an ephemeral tc/iptables sidecar container.
// Uses context.WithoutCancel with a short timeout so cleanup still runs
// when the caller's ctx was canceled by SIGTERM — otherwise pumba would
// leak the sidecar AND the rules it installed in the target's netns,
// because the caller early-returns on this error.
func (client dockerClient) removeSidecar(ctx context.Context, id string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarRemoveTimeout)
	defer cancel()
	if err := client.containerAPI.ContainerRemove(cleanupCtx, id, ctypes.RemoveOptions{Force: true}); err != nil {
		if client.sidecarRemovalComplete(ctx, id, err) {
			return nil
		}
		return err
	}
	return nil
}

func (client dockerClient) sidecarRemovalComplete(ctx context.Context, id string, removeErr error) bool {
	if cerrdefs.IsNotFound(removeErr) {
		return true
	}
	deadlineErr := cerrdefs.IsDeadlineExceeded(removeErr) ||
		errors.Is(removeErr, context.DeadlineExceeded) ||
		strings.Contains(removeErr.Error(), "context deadline exceeded")
	if !deadlineErr {
		return false
	}

	inspectCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarInspectTimeout)
	defer cancel()
	inspect, err := client.containerAPI.ContainerInspect(inspectCtx, id)
	if cerrdefs.IsNotFound(err) {
		return true
	}
	if err != nil || inspect.State == nil {
		return false
	}
	return inspect.State.Status == "removing" || inspect.State.Status == "dead" || inspect.State.Dead
}

// runSidecar launches an ephemeral sidecar container that joins target's
// network namespace, runs argsList through `tool` (tc or iptables), and is
// force-removed on completion. Used by both netem and iptables paths.
func (client dockerClient) runSidecar(ctx context.Context, target *ctr.Container, argsList [][]string, img, tool string, pull bool) error {
	log.WithFields(log.Fields{
		"container": target.ID(),
		"img":       img,
		"tool":      tool,
		"pull":      pull,
		"args-list": argsList,
	}).Debugf("executing %s command in a separate container joining target container network namespace", tool)

	hconfig := ctypes.HostConfig{
		AutoRemove:   false,
		CapAdd:       []string{"NET_ADMIN"},
		NetworkMode:  ctypes.NetworkMode("container:" + target.ID()),
		PortBindings: nat.PortMap{},
		DNS:          []string{},
		DNSOptions:   []string{},
		DNSSearch:    []string{},
	}
	log.WithField("network", hconfig.NetworkMode).Debug("network mode")
	if pull {
		if err := client.pullSidecarImage(ctx, img, tool); err != nil {
			return err
		}
	}

	// Explicit Entrypoint/Cmd keeps the sidecar alive regardless of the
	// image's default (e.g. nicolaka/netshoot defaults to zsh which exits
	// immediately in detached mode). StopSignal: SIGKILL skips the
	// SIGTERM-then-wait grace period on `rm -f`: tail as PID 1 ignores
	// SIGTERM, which otherwise makes Podman wait the full 10 s StopTimeout
	// before escalating (~tens of seconds per chaos cycle).
	config := ctypes.Config{
		Labels:     map[string]string{"com.gaiaadm.pumba.skip": "true"},
		Entrypoint: []string{"tail"},
		Cmd:        []string{"-f", "/dev/null"},
		Image:      img,
		StopSignal: "SIGKILL",
	}

	log.WithField("img", config.Image).Debugf("creating %s-container", tool)
	createResponse, err := client.containerAPI.ContainerCreate(ctx, &config, &hconfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create %s-container from %s-img: %w", tool, tool, err)
	}
	log.WithField("id", createResponse.ID).Debugf("%s container created, starting it", tool)
	if err = client.containerAPI.ContainerStart(ctx, createResponse.ID, ctypes.StartOptions{}); err != nil {
		_ = client.removeSidecar(ctx, createResponse.ID)
		return fmt.Errorf("failed to start %s-container: %w", tool, err)
	}

	for _, args := range argsList {
		if err = client.runSidecarExec(ctx, createResponse.ID, tool, args); err != nil {
			_ = client.removeSidecar(ctx, createResponse.ID)
			return fmt.Errorf("error running %s command on container: %v: %w", tool, strings.Join(args, " "), err)
		}
	}

	if err = client.removeSidecar(ctx, createResponse.ID); err != nil {
		return fmt.Errorf("failed to remove %s-container: %w", tool, err)
	}
	return nil
}

func (client dockerClient) pullSidecarImage(ctx context.Context, img, tool string) error {
	log.WithField("img", img).Debugf("pulling %s-img", tool)
	events, err := client.imageAPI.ImagePull(ctx, img, imagetypes.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull %s-img: %w", tool, err)
	}
	defer events.Close()
	d := json.NewDecoder(events)
	var resp *imagePullResponse
	for {
		if err = d.Decode(&resp); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("failed to decode docker pull response for %s-img: %w", tool, err)
		}
		log.Debug(resp)
	}
}

// runSidecarExec creates and runs an exec inside the sidecar container,
// invoking `tool` (tc or iptables) with args. The exit code is inspected so
// that a non-zero status (e.g. tc rejecting bad args, iptables rule rejected
// by kernel) surfaces as an error instead of silent success.
func (client dockerClient) runSidecarExec(ctx context.Context, sidecarID, tool string, args []string) error {
	execConfig := ctypes.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          append([]string{tool}, args...),
	}
	execCreateResponse, err := client.containerAPI.ContainerExecCreate(ctx, sidecarID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create %s-container exec: %w", tool, err)
	}
	if err := client.runExecAttached(ctx, execCreateResponse.ID); err != nil {
		return fmt.Errorf("failed to start %s-container exec: %w", tool, err)
	}
	insp, err := client.containerAPI.ContainerExecInspect(ctx, execCreateResponse.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect %s-container exec: %w", tool, err)
	}
	if insp.ExitCode != 0 {
		return fmt.Errorf("%s %s failed with exit code %d", tool, strings.Join(args, " "), insp.ExitCode)
	}
	log.WithField("args", strings.Join(args, " ")).Debugf("run command on %s-container", tool)
	return nil
}
