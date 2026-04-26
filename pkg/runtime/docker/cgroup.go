package docker

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	cgroupDriverSystemd  = "systemd"
	cgroupDriverCgroupfs = "cgroupfs"
)

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
