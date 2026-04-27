package containerd

import (
	"context"
	"fmt"

	ctr "github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// IPTablesContainer applies iptables rules to a container.
//
//nolint:dupl // intentionally parallel to StopIPTablesContainer; install/remove use identical IPTables commands on this runtime
func (c *containerdClient) IPTablesContainer(ctx context.Context, req *ctr.IPTablesRequest) error {
	log.WithField("id", req.Container.ID()).Debug("iptables on containerd container")
	if req.DryRun {
		return nil
	}
	commands := buildIPTablesCommands(req.CmdPrefix, req.CmdSuffix, req.SrcIPs, req.DstIPs, req.SPorts, req.DPorts)
	if req.Sidecar.Image != "" {
		return c.sidecarExec(ctx, req.Container, req.Sidecar.Image, req.Sidecar.Pull, "iptables", commands)
	}
	return c.runIPTablesCommands(ctx, req.Container.ID(), commands)
}

// StopIPTablesContainer removes iptables rules from a container.
//
//nolint:dupl // intentionally parallel to IPTablesContainer; install/remove use identical IPTables commands on this runtime
func (c *containerdClient) StopIPTablesContainer(ctx context.Context, req *ctr.IPTablesRequest) error {
	log.WithField("id", req.Container.ID()).Debug("stop iptables on containerd container")
	if req.DryRun {
		return nil
	}
	commands := buildIPTablesCommands(req.CmdPrefix, req.CmdSuffix, req.SrcIPs, req.DstIPs, req.SPorts, req.DPorts)
	if req.Sidecar.Image != "" {
		return c.sidecarExec(ctx, req.Container, req.Sidecar.Image, req.Sidecar.Pull, "iptables", commands)
	}
	return c.runIPTablesCommands(ctx, req.Container.ID(), commands)
}

func (c *containerdClient) runIPTablesCommands(ctx context.Context, containerID string, commands [][]string) error {
	ctx = c.nsCtx(ctx)
	for _, args := range commands {
		if err := c.execInContainer(ctx, containerID, "iptables", args); err != nil {
			return fmt.Errorf("failed to run iptables command: %w", err)
		}
	}
	return nil
}
