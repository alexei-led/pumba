package podman

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	log "github.com/sirupsen/logrus"
)

// sidecarRemoveTimeout bounds the ContainerRemove call in error cleanup paths.
// AutoRemove only fires on container exit, so attach/start failures must remove
// the created-but-never-started sidecar explicitly.
const sidecarRemoveTimeout = 10 * time.Second

// removeOnError removes a created-but-not-started sidecar so it doesn't linger
// as a dangling "created" container. AutoRemove only fires on container exit, so
// error paths after ContainerCreate must clean up explicitly.
func removeOnError(ctx context.Context, api apiBackend, id string, cause error) error {
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarRemoveTimeout)
	defer cancel()
	_ = api.ContainerRemove(rmCtx, id, ctypes.RemoveOptions{Force: true})
	return cause
}

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

	if dryrun {
		return "", nil, nil, nil
	}
	if p.rootless {
		return "", nil, nil, rootlessError("stress", p.socketURI)
	}

	cg, err := p.resolveCgroup(ctx, c.ID())
	if err != nil {
		return "", nil, nil, err
	}
	log.WithFields(log.Fields{
		"driver":     cg.driver,
		"full-path":  cg.fullPath,
		"parent":     cg.parent,
		"leaf":       cg.leaf,
		"procs-path": cg.procsPath,
	}).Debug("resolved podman target cgroup")

	config, hconfig := buildStressConfig(image, stressors, cg.driver, cg.fullPath, cg.parent, cg.procsPath, injectCgroup)

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
		return "", nil, nil, removeOnError(ctx, p.api, created.ID,
			fmt.Errorf("podman runtime: attach stress-ng container: %w", err))
	}

	if err := p.api.ContainerStart(ctx, created.ID, ctypes.StartOptions{}); err != nil {
		attach.Close()
		return "", nil, nil, removeOnError(ctx, p.api, created.ID,
			fmt.Errorf("podman runtime: start stress-ng container: %w", err))
	}

	output := make(chan string, 1)
	outerr := make(chan error, 1)
	go drainStressOutput(ctx, p.api, created.ID, attach, output, outerr)
	return created.ID, output, outerr, nil
}

// cgroupLocation bundles the host-side cgroup coordinates for a target.
type cgroupLocation struct {
	driver    string
	fullPath  string
	parent    string
	leaf      string
	procsPath string
}

// resolveCgroup locates targetID's canonical cgroup by reading /proc/<pid>/cgroup
// from the host's view. Bypasses cgroupns=private isolation that would hide
// ancestry from a process reading its own namespace-scoped view.
//
// procsPath is the concrete leaf that accepts cgroup.procs writes — either
// fullPath or fullPath/container when Podman's libpod init sub-cgroup exists.
// cgroup v2's "no internal processes" rule forbids writing to fullPath itself
// once it has a populated child, so inject-cgroup mode must target the leaf.
func (p *podmanClient) resolveCgroup(ctx context.Context, targetID string) (cgroupLocation, error) {
	info, err := p.api.ContainerInspect(ctx, targetID)
	if err != nil {
		return cgroupLocation{}, fmt.Errorf("podman runtime: inspect target %s: %w", targetID, err)
	}
	if info.State == nil || info.State.Pid == 0 {
		return cgroupLocation{}, fmt.Errorf("podman runtime: target %s is not running (no pid)", targetID)
	}
	raw, err := cgroupReader(info.State.Pid)
	if err != nil {
		return cgroupLocation{}, fmt.Errorf("podman runtime: read /proc/%d/cgroup: %w", info.State.Pid, err)
	}
	driver, fullPath, parent, leaf, err := ParseProc1Cgroup(string(raw))
	if err != nil {
		return cgroupLocation{}, fmt.Errorf("podman runtime: parse target cgroup: %w", err)
	}
	// procsPath is where cg-inject writes the sidecar's PID. Two shapes in
	// the wild:
	//   - Podman 5.x (podman machine, Fedora CoreOS): `<scope>/container`
	//     is a stable leaf holding the target PID for the container's
	//     lifetime. We must target it — the outer scope is non-leaf and
	//     write-rejected with EBUSY.
	//   - Podman 4.9.x (Ubuntu 24.04 stock): `<scope>/container` is created
	//     during libpod init, the PID migrates to `<scope>` shortly after,
	//     and `/container` is rmdir'd. /proc briefly reports `/container`,
	//     and so does os.Stat before cleanup — this function CAN'T fully
	//     close that race: the directory can vanish between os.Stat and
	//     cg-inject's write(), yielding the documented ENOENT on write.
	//
	// Reading /proc + filesystem probe is the best signal we have without
	// blocking to wait for libpod init stability. The inject-cgroup test
	// that exercises this path lives in tests/skip_ci/ on Podman 4.9.x
	// runners for that reason (see tests/podman_stress.bats). A proper
	// fix requires a retry-on-ENOENT in cg-inject itself.
	rawPath, err := RawCgroupPath(string(raw))
	if err != nil {
		return cgroupLocation{}, fmt.Errorf("podman runtime: raw target cgroup: %w", err)
	}
	procsPath := rawPath
	if _, err := os.Stat(cgroupFSRoot + rawPath); errors.Is(err, os.ErrNotExist) {
		procsPath = fullPath
	}
	return cgroupLocation{
		driver:    driver,
		fullPath:  fullPath,
		parent:    parent,
		leaf:      leaf,
		procsPath: procsPath,
	}, nil
}

// buildStressConfig returns the Config/HostConfig for the stress-ng sidecar.
//
// Default mode: place the sidecar under the target's cgroup via
// HostConfig.Resources.CgroupParent. Driver-dependent to match the Docker
// runtime's behavior:
//   - systemd: CgroupParent = parent slice (sidecar is a sibling of the target;
//     systemd rejects placing a sibling scope under another scope).
//   - cgroupfs: CgroupParent = target's full path (sidecar is a child cgroup of
//     the target, sharing limits and OOM scope — the documented contract).
//
// Explicit Entrypoint = /stress-ng mirrors the Docker runtime so custom images
// that satisfy the --stress-image contract ("/stress-ng exists") work without
// relying on image-metadata ENTRYPOINT.
//
// inject-cgroup mode: start the sidecar wherever (cgroupns=host + bind-mount
// /sys/fs/cgroup) and let /cg-inject move its PID into the target's exact
// cgroup. Required when the target's parent slice is unwritable by a sibling
// (e.g. kubelet-owned kubepods slices).
func buildStressConfig(image string, stressors []string, driver, fullPath, parent, procsPath string, injectCgroup bool) (ctypes.Config, ctypes.HostConfig) {
	labels := map[string]string{skipLabelKey: "true"}
	if injectCgroup {
		cmd := append([]string{"--cgroup-path", procsPath, "--", "/stress-ng"}, stressors...)
		return ctypes.Config{
				Image:      image,
				Labels:     labels,
				Entrypoint: []string{"/cg-inject"},
				Cmd:        cmd,
			}, ctypes.HostConfig{
				AutoRemove:   true,
				CgroupnsMode: "host",
				// CAP_SYS_ADMIN is required for cgroup v2 `cgroup.procs` writes
				// outside the sidecar's own cgroup subtree. Without it Podman
				// returns EACCES on open(...) from /cg-inject.
				CapAdd: []string{"SYS_ADMIN"},
				// Disable SELinux labeling so the sidecar's container_t domain
				// can open cgroup files under another container's scope. On
				// SELinux-enforcing hosts (Fedora CoreOS / RHEL) the default
				// type forbids cross-scope cgroup writes even with SYS_ADMIN.
				SecurityOpt: []string{"label=disable"},
				Binds:       []string{"/sys/fs/cgroup:/sys/fs/cgroup:rw"},
			}
	}
	cgroupParent := parent
	if driver == driverCgroupfs {
		cgroupParent = fullPath
	}
	return ctypes.Config{
			Image:      image,
			Labels:     labels,
			Entrypoint: []string{"/stress-ng"},
			Cmd:        stressors,
		}, ctypes.HostConfig{
			AutoRemove: true,
			Resources: ctypes.Resources{
				CgroupParent: cgroupParent,
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
