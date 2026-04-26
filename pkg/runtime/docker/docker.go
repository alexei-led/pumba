package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	log "github.com/sirupsen/logrus"
)

type imagePullResponse struct {
	Status         string `json:"status"`
	Error          string `json:"error"`
	Progress       string `json:"progress"`
	ProgressDetail struct {
		Current int `json:"current"`
		Total   int `json:"total"`
	} `json:"progressDetail"`
}

const cgroupDriverSystemd = "systemd"
const cgroupDriverCgroupfs = "cgroupfs"

// StressContainer starts stress test on a container (CPU, memory, network, io)
func (client dockerClient) StressContainer(ctx context.Context, c *ctr.Container, stressors []string, img string, pull bool, duration time.Duration, injectCgroup, dryrun bool) (string, <-chan string, <-chan error, error) {
	log.WithFields(log.Fields{
		"name":          c.Name(),
		"id":            c.ID(),
		"stressors":     stressors,
		"img":           img,
		"pull":          pull,
		"duration":      duration,
		"inject-cgroup": injectCgroup,
		"dryrun":        dryrun,
	}).Info("stress testing container")
	if dryrun {
		return "", nil, nil, nil
	}
	return client.stressContainerCommand(ctx, c.ID(), stressors, img, pull, injectCgroup)
}

// cgroupDriver queries the Docker daemon for its cgroup driver.
// Returns the driver name or empty string with error on failure.
func (client dockerClient) cgroupDriver(ctx context.Context) (string, error) {
	info, err := client.systemAPI.Info(ctx)
	if err != nil {
		log.WithError(err).Warn("failed to get docker info, assuming cgroupfs driver")
		return "", err
	}
	return info.CgroupDriver, nil
}

// containerLeafCgroup returns the leaf cgroup directory name for a container
// based on the cgroup driver. On cgroupfs the leaf is the container ID; on
// systemd it is a scope unit named "docker-<id>.scope".
func containerLeafCgroup(targetID, driver string) string {
	if driver == cgroupDriverSystemd {
		return "docker-" + targetID + ".scope"
	}
	return targetID
}

// inspectCgroupParent returns the target container's CgroupParent from inspect.
// Returns empty string when CgroupParent is not set (standalone Docker defaults)
// or when inspect fails.
func (client dockerClient) inspectCgroupParent(ctx context.Context, targetID string) string {
	inspect, err := client.containerAPI.ContainerInspect(ctx, targetID)
	if err != nil {
		log.WithError(err).Warn("failed to inspect target container for cgroup path")
		return ""
	}
	if inspect.HostConfig != nil && inspect.HostConfig.CgroupParent != "" {
		log.WithField("cgroup-parent", inspect.HostConfig.CgroupParent).Debug("resolved cgroup parent from container inspect")
		return inspect.HostConfig.CgroupParent
	}
	return ""
}

// defaultCgroupParent returns the default cgroup parent path based on the Docker
// daemon's cgroup driver when the target container has no explicit CgroupParent set.
func defaultCgroupParent(targetID, driver string) string {
	switch driver {
	case cgroupDriverSystemd:
		return "system.slice"
	default:
		return "/docker/" + targetID
	}
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

// pullImage pulls a Docker image and drains the progress stream.
func (client dockerClient) pullImage(ctx context.Context, img string) error {
	log.WithField("img", img).Debug("pulling image")
	events, err := client.imageAPI.ImagePull(ctx, img, imagetypes.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull stress-ng img: %w", err)
	}
	defer events.Close()
	d := json.NewDecoder(events)
	var pullResponse *imagePullResponse
	for {
		if err = d.Decode(&pullResponse); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("failed to decode docker pull result: %w", err)
		}
		log.Debug(pullResponse)
	}
}

// stressResolveDriver resolves the cgroup driver, parent, and target cgroup path
// for stress container setup. For default mode, cgroupParent is the resolved path
// for --cgroup-parent. For inject-cgroup mode, cgroupPath is the target's full
// cgroup path (if known) to pass as --cgroup-path to cg-inject.
func (client dockerClient) stressResolveDriver(ctx context.Context, targetID string, injectCgroup bool) (driver, cgroupParent, cgroupPath string, err error) {
	// resolve the cgroup driver first — needed for correct leaf cgroup naming
	driver, err = client.cgroupDriver(ctx)
	if err != nil {
		// try inspect anyway; if it yields a parent we can still proceed
		inspectParent := client.inspectCgroupParent(ctx, targetID)
		if inspectParent == "" {
			return "", "", "", fmt.Errorf("failed to get docker info: %w", err)
		}
		// infer driver from parent path: systemd parents end with .slice
		if strings.HasSuffix(inspectParent, ".slice") {
			driver = cgroupDriverSystemd
		} else {
			driver = cgroupDriverCgroupfs
		}
		cgroupPath = inspectParent + "/" + containerLeafCgroup(targetID, driver)
	} else {
		if driver == "" {
			driver = cgroupDriverCgroupfs
		}
		if inspectParent := client.inspectCgroupParent(ctx, targetID); inspectParent != "" {
			cgroupPath = inspectParent + "/" + containerLeafCgroup(targetID, driver)
		}
	}

	if injectCgroup {
		return driver, cgroupParent, cgroupPath, nil
	}
	if cgroupPath == "" {
		cgroupParent = defaultCgroupParent(targetID, driver)
		return driver, cgroupParent, cgroupPath, nil
	}
	// For default mode, CgroupParent must be a value Docker accepts.
	// systemd requires a valid slice name (*.slice); cgroupfs accepts any path.
	if driver == cgroupDriverSystemd {
		cgroupParent = cgroupPath[:strings.LastIndex(cgroupPath, "/")]
	} else {
		cgroupParent = cgroupPath
	}
	return driver, cgroupParent, cgroupPath, nil
}

// execute a stress-ng command in stress-ng Docker container in target container cgroup
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
		return createResponse.ID, output, outerr, fmt.Errorf("failed to start stress-ng container: %w", err)
	}
	return createResponse.ID, output, outerr, nil
}
