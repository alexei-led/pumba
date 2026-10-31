package containerd

import (
	"context"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
	log "github.com/sirupsen/logrus"
)

// NetemContainer applies ownership-safe network emulation to a container.
func (c *containerdClient) NetemContainer(ctx context.Context, req *ctr.NetemRequest) error {
	log.WithFields(log.Fields{"id": req.Container.ID(), "interface": req.Interface, "tc-image": req.Sidecar.Image}).Debug("netem on containerd container")
	if req.DryRun {
		return nil
	}
	return c.runNetemCommand(ctx, req, netns.NetemStart(req))
}

// StopNetemContainer removes only a verified Pumba-owned netem topology.
func (c *containerdClient) StopNetemContainer(ctx context.Context, req *ctr.NetemRequest) error {
	log.WithFields(log.Fields{"id": req.Container.ID(), "interface": req.Interface, "tc-image": req.Sidecar.Image}).Debug("stop netem on containerd container")
	if req.DryRun {
		return nil
	}
	return c.runNetemCommand(ctx, req, netns.NetemStop(req))
}

// PlanNetemContainer returns the argv NetemContainer and StopNetemContainer
// run for req.
func (c *containerdClient) PlanNetemContainer(req *ctr.NetemRequest) (*ctr.CommandPlan, error) {
	return netns.NetemPlan(req), nil
}

func (c *containerdClient) runNetemCommand(ctx context.Context, req *ctr.NetemRequest, argv []string) error {
	if req.Sidecar.Image != "" {
		labels := ctr.SidecarLabels(req.RunID, req.Container.ID(), ctr.SidecarKindNetem)
		return c.sidecarExec(ctx, req.Container, req.Sidecar, labels, [][]string{argv})
	}
	return c.execInContainer(c.nsCtx(ctx), req.Container.ID(), argv[0], argv[1:])
}
