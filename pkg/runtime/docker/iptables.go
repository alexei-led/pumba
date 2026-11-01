package docker

import (
	"context"
	"fmt"
	"strings"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
	tcplan "github.com/alexei-led/pumba/pkg/tc"
	log "github.com/sirupsen/logrus"
)

// IPTablesContainer injects sidecar iptables container into the given container network namespace
func (client dockerClient) IPTablesContainer(ctx context.Context, req *ctr.IPTablesRequest) error {
	logIPTablesRequest(req).Info("running iptables on container")
	return tcplan.ExplainIPTablesError(client.runIPTables(ctx, req), req.RunID != "")
}

// StopIPTablesContainer stops the iptables container injected into the given container network namespace
func (client dockerClient) StopIPTablesContainer(ctx context.Context, req *ctr.IPTablesRequest) error {
	logIPTablesRequest(req).Info("stopping iptables on container")
	return client.runIPTables(ctx, req)
}

func logIPTablesRequest(req *ctr.IPTablesRequest) *log.Entry {
	return log.WithFields(log.Fields{
		"name":          req.Container.Name(),
		"id":            req.Container.ID(),
		"commandPrefix": req.CmdPrefix,
		"commandSuffix": req.CmdSuffix,
		"srcIPs":        req.SrcIPs,
		"dstIPs":        req.DstIPs,
		"sports":        req.SPorts,
		"dports":        req.DPorts,
		"duration":      req.Duration,
		"img":           req.Sidecar.Image,
		"pull":          req.Sidecar.Pull,
		"run-id":        req.RunID,
		"dryrun":        req.DryRun,
	})
}

func (client dockerClient) runIPTables(ctx context.Context, req *ctr.IPTablesRequest) error {
	if req.DryRun {
		return nil
	}
	commands := netns.IPTablesCommands(req)
	if req.Sidecar.Image != "" {
		labels := ctr.SidecarLabels(req.RunID, req.Container.ID(), ctr.SidecarKindIPTables)
		return client.runSidecar(ctx, req.Container, req.Sidecar, labels, "iptables", commands)
	}
	for _, args := range commands {
		log.WithField("iptables", strings.Join(args, " ")).Debug("executing iptables")
		if err := client.execOnContainer(ctx, req.Container, "iptables", args, true); err != nil {
			return fmt.Errorf("error running iptables command on container: %v: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}
