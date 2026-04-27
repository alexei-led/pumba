package containerd

import (
	"context"
	"fmt"

	ctr "github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// NetemContainer applies network emulation to a container by executing tc commands.
func (c *containerdClient) NetemContainer(ctx context.Context, req *ctr.NetemRequest) error {
	log.WithFields(log.Fields{"id": req.Container.ID(), "interface": req.Interface, "tc-image": req.Sidecar.Image}).Debug("netem on containerd container")
	if req.DryRun {
		return nil
	}
	tcCommands := buildNetemCommands(req.Interface, req.Command, req.IPs, req.SPorts, req.DPorts)
	if req.Sidecar.Image != "" {
		return c.sidecarExec(ctx, req.Container, req.Sidecar.Image, req.Sidecar.Pull, "tc", tcCommands)
	}
	return c.runTCCommands(c.nsCtx(ctx), req.Container.ID(), tcCommands)
}

// StopNetemContainer removes network emulation from a container.
func (c *containerdClient) StopNetemContainer(ctx context.Context, req *ctr.NetemRequest) error {
	log.WithFields(log.Fields{"id": req.Container.ID(), "interface": req.Interface, "tc-image": req.Sidecar.Image}).Debug("stop netem on containerd container")
	if req.DryRun {
		return nil
	}
	hasFilters := len(req.IPs) > 0 || len(req.SPorts) > 0 || len(req.DPorts) > 0
	tcCommands := buildStopNetemCommands(req.Interface, hasFilters)
	if req.Sidecar.Image != "" {
		return c.sidecarExec(ctx, req.Container, req.Sidecar.Image, req.Sidecar.Pull, "tc", tcCommands)
	}
	return c.runTCCommands(c.nsCtx(ctx), req.Container.ID(), tcCommands)
}

func (c *containerdClient) runTCCommands(ctx context.Context, containerID string, commands [][]string) error {
	for _, args := range commands {
		if err := c.execInContainer(ctx, containerID, "tc", args); err != nil {
			return fmt.Errorf("failed to run tc command: %w", err)
		}
	}
	return nil
}
