package podman

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	log "github.com/sirupsen/logrus"
)

// skipLabelKey tags pumba-spawned sidecars so list filters can exclude them
// from chaos target selection. Shared with the docker runtime.
const skipLabelKey = "com.gaiaadm.pumba.skip"

// StressContainer launches a stress-ng sidecar that targets c's cgroup. The
// target cgroup is resolved host-side from /proc/<pid>/cgroup — necessary
// because modern Podman defaults to private cgroup namespaces that hide the
// full ancestry from a process reading its own /proc/self/cgroup.
//
// Rootless Podman cannot create a child cgroup under the target's scope (the
// kernel denies the write) and cannot grant the sidecar the CAP_SYS_ADMIN
// that cg-inject needs, so fail fast with rootlessError rather than surface
// an opaque sidecar create/start failure.
func (p *podmanClient) StressContainer(ctx context.Context, c *ctr.Container, stressors []string, image string, pull bool, duration time.Duration, injectCgroup, dryrun bool) (string, <-chan string, <-chan error, error) {
	log.WithFields(log.Fields{
		"name":          c.Name(),
		"id":            c.ID(),
		"stressors":     stressors,
		"image":         image,
		"pull":          pull,
		"duration":      duration,
		"inject-cgroup": injectCgroup,
		"dryrun":        dryrun,
	}).Info("stress testing podman container")

	if p.rootless {
		return "", nil, nil, rootlessError("stress", p.socketURI)
	}
	if dryrun {
		return "", nil, nil, nil
	}

	driver, fullPath, parent, leaf, err := p.resolveCgroup(ctx, c.ID())
	if err != nil {
		return "", nil, nil, err
	}
	log.WithFields(log.Fields{
		"driver":    driver,
		"full-path": fullPath,
		"parent":    parent,
		"leaf":      leaf,
	}).Debug("resolved podman target cgroup")

	config, hconfig := buildStressConfig(image, stressors, fullPath, parent, injectCgroup)

	if pull {
		if err := p.pullStressImage(ctx, image); err != nil {
			return "", nil, nil, err
		}
	}

	created, err := p.api.ContainerCreate(ctx, &config, &hconfig, nil, nil, "")
	if err != nil {
		return "", nil, nil, fmt.Errorf("podman runtime: create stress-ng container: %w", err)
	}

	attach, err := p.api.ContainerAttach(ctx, created.ID, ctypes.AttachOptions{
		Stdout: true,
		Stderr: true,
		Stream: true,
	})
	if err != nil {
		return "", nil, nil, fmt.Errorf("podman runtime: attach stress-ng container: %w", err)
	}

	output := make(chan string, 1)
	outerr := make(chan error, 1)
	go drainStressOutput(ctx, p.api, created.ID, attach, output, outerr)

	if err := p.api.ContainerStart(ctx, created.ID, ctypes.StartOptions{}); err != nil {
		return created.ID, output, outerr, fmt.Errorf("podman runtime: start stress-ng container: %w", err)
	}
	return created.ID, output, outerr, nil
}

// resolveCgroup locates targetID's canonical cgroup by reading /proc/<pid>/cgroup
// from the host's view. Bypasses cgroupns=private isolation that would hide
// ancestry from a process reading its own namespace-scoped view.
func (p *podmanClient) resolveCgroup(ctx context.Context, targetID string) (driver, fullPath, parent, leaf string, err error) {
	info, err := p.api.ContainerInspect(ctx, targetID)
	if err != nil {
		return "", "", "", "", fmt.Errorf("podman runtime: inspect target %s: %w", targetID, err)
	}
	if info.State == nil || info.State.Pid == 0 {
		return "", "", "", "", fmt.Errorf("podman runtime: target %s is not running (no pid)", targetID)
	}
	raw, err := cgroupReader(info.State.Pid)
	if err != nil {
		return "", "", "", "", fmt.Errorf("podman runtime: read /proc/%d/cgroup: %w", info.State.Pid, err)
	}
	driver, fullPath, parent, leaf, err = ParseProc1Cgroup(string(raw))
	if err != nil {
		return "", "", "", "", fmt.Errorf("podman runtime: parse target cgroup: %w", err)
	}
	return driver, fullPath, parent, leaf, nil
}

// buildStressConfig returns the Config/HostConfig for the stress-ng sidecar.
//
// Default mode: place the sidecar as a sibling leaf under the target's
// parent cgroup via HostConfig.Resources.CgroupParent. Podman honors this
// on the Docker-compat socket the same way Docker does.
//
// inject-cgroup mode: start the sidecar wherever (cgroupns=host + bind-mount
// /sys/fs/cgroup) and let /cg-inject move its PID into the target's exact
// cgroup. Required when the target's parent slice is unwritable by a sibling
// (e.g. kubelet-owned kubepods slices).
func buildStressConfig(image string, stressors []string, fullPath, parent string, injectCgroup bool) (ctypes.Config, ctypes.HostConfig) {
	labels := map[string]string{skipLabelKey: "true"}
	if injectCgroup {
		cmd := append([]string{"--cgroup-path", fullPath, "--", "/stress-ng"}, stressors...)
		return ctypes.Config{
				Image:      image,
				Labels:     labels,
				Entrypoint: []string{"/cg-inject"},
				Cmd:        cmd,
			}, ctypes.HostConfig{
				AutoRemove:   true,
				CgroupnsMode: "host",
				Binds:        []string{"/sys/fs/cgroup:/sys/fs/cgroup:rw"},
			}
	}
	return ctypes.Config{
			Image:  image,
			Labels: labels,
			Cmd:    stressors,
		}, ctypes.HostConfig{
			AutoRemove: true,
			Resources: ctypes.Resources{
				CgroupParent: parent,
			},
		}
}

// pullStressImage pulls img and drains the progress stream to completion so
// the subsequent ContainerCreate sees the layers committed locally.
func (p *podmanClient) pullStressImage(ctx context.Context, img string) error {
	log.WithField("image", img).Debug("pulling stress-ng image via podman")
	events, err := p.api.ImagePull(ctx, img, imagetypes.PullOptions{})
	if err != nil {
		return fmt.Errorf("podman runtime: pull stress-ng image %s: %w", img, err)
	}
	defer events.Close()
	dec := json.NewDecoder(events)
	for {
		var response imagePullResponse
		if err := dec.Decode(&response); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("podman runtime: decode pull stream: %w", err)
		}
		log.Debug(response)
	}
}

// imagePullResponse matches the JSON stream emitted by the Docker-compat
// image pull endpoint. Used only to drain the channel — fields are logged
// verbatim, never branched on.
type imagePullResponse struct {
	Status   string `json:"status"`
	Error    string `json:"error"`
	Progress string `json:"progress"`
}

// drainStressOutput is the stress sidecar's attach-reader loop. Mirrors the
// docker.go pattern: copy the muxed stdout/stderr frame stream to a buffer,
// inspect the container for its exit code once the reader hits EOF, and emit
// either the captured buffer on success or a wrapped error on non-zero exit.
// The buffer is diagnostic only — it still contains 8-byte stream headers
// from Docker's frame protocol and is not meant to be parsed programmatically.
func drainStressOutput(ctx context.Context, api apiBackend, id string, attach types.HijackedResponse, output chan<- string, outerr chan<- error) {
	defer close(output)
	defer close(outerr)
	defer attach.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, attach.Reader); err != nil {
		outerr <- fmt.Errorf("podman runtime: drain stress-ng stdout: %w", err)
		return
	}
	inspect, err := api.ContainerInspect(ctx, id)
	if err != nil {
		outerr <- fmt.Errorf("podman runtime: inspect stress-ng after exit: %w", err)
		return
	}
	if inspect.State != nil && inspect.State.ExitCode != 0 {
		outerr <- fmt.Errorf("podman runtime: stress-ng exited with code %d: %s", inspect.State.ExitCode, buf.String())
		return
	}
	output <- buf.String()
}
