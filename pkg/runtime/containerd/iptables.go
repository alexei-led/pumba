package containerd

import (
	"context"
	"fmt"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
	tcplan "github.com/alexei-led/pumba/pkg/tc"
	log "github.com/sirupsen/logrus"
)

// IPTablesContainer applies iptables rules to a container.
func (c *containerdClient) IPTablesContainer(ctx context.Context, req *ctr.IPTablesRequest) error {
	log.WithField("id", req.Container.ID()).Debug("iptables on containerd container")
	return tcplan.ExplainIPTablesError(c.runIPTables(ctx, req), req.RunID != "")
}

// StopIPTablesContainer removes iptables rules from a container.
func (c *containerdClient) StopIPTablesContainer(ctx context.Context, req *ctr.IPTablesRequest) error {
	log.WithField("id", req.Container.ID()).Debug("stop iptables on containerd container")
	return c.runIPTables(ctx, req)
}

// PlanIPTablesContainer returns the argv IPTablesContainer(add) and
// StopIPTablesContainer(del) run.
func (c *containerdClient) PlanIPTablesContainer(add, del *ctr.IPTablesRequest) (*ctr.CommandPlan, error) {
	return netns.IPTablesPlan(add, del)
}

func (c *containerdClient) runIPTables(ctx context.Context, req *ctr.IPTablesRequest) error {
	if req.DryRun {
		return nil
	}
	commands, err := netns.IPTablesCommands(req)
	if err != nil {
		return err
	}
	if req.Sidecar.Image != "" {
		labels := ctr.SidecarLabels(req.RunID, req.Container.ID(), ctr.SidecarKindIPTables)
		return c.sidecarExec(ctx, req.Container, req.Sidecar, labels, commands)
	}
	ctx = c.nsCtx(ctx)
	for _, argv := range commands {
		if err := c.execFaultInContainer(ctx, req.Container.ID(), argv[0], argv[1:]); err != nil {
			return fmt.Errorf("failed to run iptables command: %w", err)
		}
	}
	return nil
}
