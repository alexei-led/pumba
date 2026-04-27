package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"

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
	id, output, errCh, err := client.stressContainerCommand(ctx, req.Container.ID(), req.Stressors, req.Sidecar.Image, req.Sidecar.Pull, req.InjectCgroup)
	if err != nil {
		return nil, err
	}
	return &ctr.StressResult{SidecarID: id, Output: output, Errors: errCh}, nil
}

// stressContainerConfig builds the container and host config for a stress-ng container.
// cgroupPath is the target's cgroup base path resolved from ContainerInspect (may be empty).
// For inject-cgroup mode: when cgroupPath is known, uses --cgroup-path; otherwise falls back
// to --target-id + --cgroup-driver.
func stressContainerConfig(targetID string, stressors []string, img, driver, cgroupParent, cgroupPath string, injectCgroup bool) (ctypes.Config, ctypes.HostConfig) {
	if injectCgroup {
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
				Labels:     map[string]string{"com.gaiaadm.pumba.skip": "true"},
				Image:      img,
				Entrypoint: []string{"/cg-inject"},
				Cmd:        cmd,
			}, ctypes.HostConfig{
				AutoRemove:   true,
				CgroupnsMode: "host",
				Binds:        []string{"/sys/fs/cgroup:/sys/fs/cgroup:rw"},
			}
	}
	// default child-cgroup mode: use --cgroup-parent with the resolved path
	log.WithField("cgroup-parent", cgroupParent).Debug("resolved cgroup parent")
	return ctypes.Config{
			Labels:     map[string]string{"com.gaiaadm.pumba.skip": "true"},
			Image:      img,
			Entrypoint: []string{"/stress-ng"},
			Cmd:        stressors,
		}, ctypes.HostConfig{
			AutoRemove: true,
			Resources: ctypes.Resources{
				CgroupParent: cgroupParent,
			},
		}
}

// stressContainerCommand executes a stress-ng command in a stress-ng Docker container
// in the target container's cgroup.
func (client dockerClient) stressContainerCommand(ctx context.Context, targetID string, stressors []string, img string, pull, injectCgroup bool) (string, <-chan string, <-chan error, error) {
	log.WithFields(log.Fields{
		"target":        targetID,
		"stressors":     stressors,
		"img":           img,
		"pull":          pull,
		"inject-cgroup": injectCgroup,
	}).Debug("executing stress-ng command")

	driver, cgroupParent, cgroupPath, err := client.stressResolveDriver(ctx, targetID, injectCgroup)
	if err != nil {
		return "", nil, nil, err
	}

	config, hconfig := stressContainerConfig(targetID, stressors, img, driver, cgroupParent, cgroupPath, injectCgroup)
	if pull {
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
		// AutoRemove fires only on container exit; a never-attached container
		// is leaked unless we remove it explicitly.
		if rmErr := client.removeSidecar(ctx, createResponse.ID); rmErr != nil {
			log.WithError(rmErr).WithField("id", createResponse.ID).Warn("failed to remove never-attached stress-ng sidecar")
		}
		return "", nil, nil, fmt.Errorf("failed to attach to stress-ng container: %w", err)
	}
	output := make(chan string, 1)
	outerr := make(chan error, 1)
	// copy stderr and stdout from attached reader
	go func() {
		defer close(output)
		defer close(outerr)
		defer attach.Close()
		var stdout bytes.Buffer
		_, e := io.Copy(&stdout, attach.Reader)
		if e != nil {
			outerr <- e
			return
		}
		// inspect stress-ng container
		inspect, e := client.containerAPI.ContainerInspect(ctx, createResponse.ID)
		if e != nil {
			outerr <- fmt.Errorf("failed to inspect stress-ng container: %w", e)
			return
		}
		// get status of stress-ng command
		if inspect.State.ExitCode != 0 {
			outerr <- fmt.Errorf("stress-ng exited with error: %v", stdout.String())
			return
		}
		output <- stdout.String()
	}()
	// start stress-ng container running stress-ng in target container cgroup
	log.WithField("id", createResponse.ID).Debug("stress-ng container created, starting it")
	err = client.containerAPI.ContainerStart(ctx, createResponse.ID, ctypes.StartOptions{})
	if err != nil {
		// AutoRemove fires only on container exit; a never-started container
		// is leaked unless we remove it explicitly.
		if rmErr := client.removeSidecar(ctx, createResponse.ID); rmErr != nil {
			log.WithError(rmErr).WithField("id", createResponse.ID).Warn("failed to remove never-started stress-ng sidecar")
		}
		return createResponse.ID, output, outerr, fmt.Errorf("failed to start stress-ng container: %w", err)
	}
	return createResponse.ID, output, outerr, nil
}
