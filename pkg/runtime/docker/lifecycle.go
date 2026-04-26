package docker

import (
	"context"
	"errors"
	"fmt"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	log "github.com/sirupsen/logrus"
)

const (
	defaultStopSignal = "SIGTERM"
	defaultKillSignal = "SIGKILL"
)

// KillContainer kills a container with the given signal
func (client dockerClient) KillContainer(ctx context.Context, c *ctr.Container, signal string, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"signal": signal,
		"dryrun": dryrun,
	}).Info("killing container")
	if dryrun {
		return nil
	}
	err := client.containerAPI.ContainerKill(ctx, c.ID(), signal)
	if err != nil {
		return fmt.Errorf("failed to kill container: %w", err)
	}
	return nil
}

// RestartContainer restarts a container
func (client dockerClient) RestartContainer(ctx context.Context, c *ctr.Container, timeout time.Duration, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"timeout": timeout,
		"dryrun":  dryrun,
	}).Info("restart container")
	if dryrun {
		return nil
	}
	timeoutSec := int(timeout.Seconds())
	if err := client.containerAPI.ContainerRestart(ctx, c.ID(), ctypes.StopOptions{Timeout: &timeoutSec}); err != nil {
		return fmt.Errorf("failed to restart container: %w", err)
	}
	return nil
}

// StopContainer stops a container
func (client dockerClient) StopContainer(ctx context.Context, c *ctr.Container, timeout int, dryrun bool) error {
	signal := c.StopSignal()
	if signal == "" {
		signal = defaultStopSignal
	}
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"timout": timeout,
		"signal": signal,
		"dryrun": dryrun,
	}).Info("stopping container")
	if dryrun {
		return nil
	}
	if err := client.containerAPI.ContainerKill(ctx, c.ID(), signal); err != nil {
		return fmt.Errorf("failed to kill container: %w", err)
	}

	// Wait for container to exit, but proceed anyway after the timeout elapses
	if err := client.waitForStop(ctx, c, timeout); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"name":    c.Name(),
			"id":      c.ID(),
			"timeout": timeout,
		}).Warn("failed waiting for container to stop, going to kill it")

		// failed to stop gracefully - going to kill target container
		log.WithFields(log.Fields{
			"name":   c.Name(),
			"id":     c.ID(),
			"signal": defaultKillSignal,
		}).Debug("killing container")
		if err := client.containerAPI.ContainerKill(ctx, c.ID(), defaultKillSignal); err != nil {
			return fmt.Errorf("failed to kill container: %w", err)
		}
		// Wait for container to be removed
		if err := client.waitForStop(ctx, c, timeout); err != nil {
			return errors.New("failed waiting for container to stop")
		}
	}
	return nil
}

// StopContainerWithID stops a container with a timeout
func (client dockerClient) StopContainerWithID(ctx context.Context, containerID string, timeout time.Duration, dryrun bool) error {
	log.WithFields(log.Fields{
		"id":      containerID,
		"timeout": timeout,
		"dryrun":  dryrun,
	}).Info("stopping container")
	if dryrun {
		return nil
	}
	timeoutSec := int(timeout.Seconds())
	err := client.containerAPI.ContainerStop(ctx, containerID, ctypes.StopOptions{Timeout: &timeoutSec})
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	return nil
}

// StartContainer starts a container
func (client dockerClient) StartContainer(ctx context.Context, c *ctr.Container, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"dryrun": dryrun,
	}).Info("starting container")
	if dryrun {
		return nil
	}
	err := client.containerAPI.ContainerStart(ctx, c.ID(), ctypes.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	return nil
}

// RemoveContainer removes a container
func (client dockerClient) RemoveContainer(ctx context.Context, c *ctr.Container, force, links, volumes, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"force":   force,
		"links":   links,
		"volumes": volumes,
		"dryrun":  dryrun,
	}).Info("removing container")
	if dryrun {
		return nil
	}
	removeOpts := ctypes.RemoveOptions{
		RemoveVolumes: volumes,
		RemoveLinks:   links,
		Force:         force,
	}
	err := client.containerAPI.ContainerRemove(ctx, c.ID(), removeOpts)
	if err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}
	return nil
}

// PauseContainer pauses a container main process
func (client dockerClient) PauseContainer(ctx context.Context, c *ctr.Container, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"dryrun": dryrun,
	}).Info("pausing container")
	if dryrun {
		return nil
	}
	err := client.containerAPI.ContainerPause(ctx, c.ID())
	if err != nil {
		return fmt.Errorf("failed to pause container: %w", err)
	}
	return nil
}

// UnpauseContainer unpauses a container main process
func (client dockerClient) UnpauseContainer(ctx context.Context, c *ctr.Container, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"dryrun": dryrun,
	}).Info("stop pausing container")
	if dryrun {
		return nil
	}
	err := client.containerAPI.ContainerUnpause(ctx, c.ID())
	if err != nil {
		return fmt.Errorf("failed to unpause container: %w", err)
	}
	return nil
}

func (client dockerClient) waitForStop(ctx context.Context, c *ctr.Container, waitTime int) error {
	// check status every 100 ms
	const checkInterval = 100 * time.Millisecond
	// timeout after waitTime seconds
	timeout := time.After(time.Duration(waitTime) * time.Second)
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"timeout": timeout,
	}).Debug("waiting for container to stop")
	for {
		select {
		case <-timeout:
			return errors.New("timeout on waiting to stop")
		case <-ctx.Done():
			return errors.New("aborted waiting to stop")
		default:
			if ci, err := client.containerAPI.ContainerInspect(ctx, c.ID()); err != nil {
				return fmt.Errorf("failed to inspect container, while waiting to stop: %w", err)
			} else if !ci.State.Running {
				return nil
			}
		}
		time.Sleep(checkInterval)
	}
}
