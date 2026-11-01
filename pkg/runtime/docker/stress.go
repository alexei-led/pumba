package docker

import (
	"context"
	"errors"
	"fmt"

	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	log "github.com/sirupsen/logrus"
)

// StressContainer starts stress test on a container (CPU, memory, network, io)
func (client dockerClient) StressContainer(ctx context.Context, req *ctr.StressRequest) (*ctr.StressResult, error) {
	log.WithFields(log.Fields{
		"name":          req.Container.Name(),
		"id":            req.Container.ID(),
		"stressors":     req.Stressors,
		"img":           req.Sidecar.Image,
		"pull":          req.Sidecar.Pull,
		"duration":      req.Duration,
		"inject-cgroup": req.InjectCgroup,
		"dryrun":        req.DryRun,
	}).Info("stress testing container")
	if req.DryRun {
		return &ctr.StressResult{}, nil
	}
	id, output, errCh, err := client.stressContainerCommand(ctx, req)
	if err != nil {
		return nil, err
	}
	return &ctr.StressResult{SidecarID: id, Output: output, Errors: errCh}, nil
}

// stressContainerConfig builds the container and host config for a stress-ng container.
// cgroupPath is the target's cgroup base path resolved from ContainerInspect (may be empty).
// For inject-cgroup mode: when cgroupPath is known, uses --cgroup-path; otherwise falls back
// to --target-id + --cgroup-driver.
func stressContainerConfig(req *ctr.StressRequest, driver, cgroupParent, cgroupPath string) (ctypes.Config, ctypes.HostConfig) {
	targetID, stressors, img := req.Container.ID(), req.Stressors, req.Sidecar.Image
	labels := ctr.SidecarLabels(req.RunID, targetID, ctr.SidecarKindStress)
	if req.InjectCgroup {
		var cmd []string
		if cgroupPath != "" {
			cmd = append([]string{"--cgroup-path", cgroupPath, "--", "/stress-ng"}, stressors...)
			log.WithField("cgroup-path", cgroupPath).Debug("using inject-cgroup mode with explicit cgroup path")
		} else {
			cmd = append([]string{"--target-id", targetID, "--cgroup-driver", driver, "--", "/stress-ng"}, stressors...)
			log.WithFields(log.Fields{
				"driver":    driver,
				"target-id": targetID,
			}).Debug("using inject-cgroup mode with driver-based path")
		}
		return ctypes.Config{
			Labels:     labels,
			Image:      img,
			Entrypoint: []string{"/cg-inject"},
			Cmd:        cmd,
		}, ctypes.HostConfig{
			AutoRemove:   false,
			CgroupnsMode: "host",
			Binds:        []string{"/sys/fs/cgroup:/sys/fs/cgroup:rw"},
		}
	}
	// default child-cgroup mode: use --cgroup-parent with the resolved path
	log.WithField("cgroup-parent", cgroupParent).Debug("resolved cgroup parent")
	return ctypes.Config{
		Labels:     labels,
		Image:      img,
		Entrypoint: []string{"/stress-ng"},
		Cmd:        stressors,
	}, ctypes.HostConfig{
		AutoRemove:   false,
		CgroupParent: cgroupParent,
	}
}

// stressContainerCommand executes a stress-ng command in a stress-ng Docker container
// in the target container's cgroup.
func (client dockerClient) stressContainerCommand(ctx context.Context, req *ctr.StressRequest) (string, <-chan string, <-chan error, error) {
	log.WithFields(log.Fields{
		"target":        req.Container.ID(),
		"stressors":     req.Stressors,
		"img":           req.Sidecar.Image,
		"pull":          req.Sidecar.Pull,
		"inject-cgroup": req.InjectCgroup,
	}).Debug("executing stress-ng command")

	driver, cgroupParent, cgroupPath, err := client.stressResolveDriver(ctx, req.Container.ID(), req.InjectCgroup)
	if err != nil {
		return "", nil, nil, err
	}

	config, hconfig := stressContainerConfig(req, driver, cgroupParent, cgroupPath)
	if req.Sidecar.Pull {
		if err := client.pullImage(ctx, config.Image); err != nil {
			return "", nil, nil, err
		}
	}
	// create stress-ng container
	log.WithField("img", config.Image).Debug("creating stress-ng container")
	createResponse, err := client.containerAPI.ContainerCreate(ctx, &config, &hconfig, nil, nil, "")
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to create stress-ng container: %w", err)
	}
	// attach to stress-ng container, capturing stdout and stderr
	opts := ctypes.AttachOptions{
		Stdin:  false,
		Stdout: true,
		Stderr: true,
		Stream: true,
	}
	attach, err := client.containerAPI.ContainerAttach(ctx, createResponse.ID, opts)
	if err != nil {
		return "", nil, nil, errors.Join(fmt.Errorf("failed to attach to stress-ng container: %w", err), client.removeStressSidecar(ctx, createResponse.ID))
	}
	// Start before launching the reader or observing exit. A failed start must
	// close the attachment without leaving a goroutine blocked in io.Copy.
	if err := client.containerAPI.ContainerStart(ctx, createResponse.ID, ctypes.StartOptions{}); err != nil {
		attach.Close()
		return createResponse.ID, nil, nil, errors.Join(fmt.Errorf("failed to start stress-ng container: %w", err), client.removeStressSidecar(ctx, createResponse.ID))
	}
	output := make(chan string, 1)
	outerr := make(chan error, 1)
	go client.drainStressOutput(ctx, createResponse.ID, attach, output, outerr)
	return createResponse.ID, output, outerr, nil
}
