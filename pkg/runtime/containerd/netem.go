package containerd

import (
	"context"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
	containerd "github.com/containerd/containerd/v2/client"
	log "github.com/sirupsen/logrus"
)

// NetemContainer applies ownership-safe network emulation to a container.
func (c *containerdClient) NetemContainer(ctx context.Context, req *ctr.NetemRequest) error {
	log.WithFields(log.Fields{"id": req.Container.ID(), "interface": req.Interface, "tc-image": req.Sidecar.Image}).Debug("netem on containerd container")
	if req.DryRun {
		return nil
	}
	argv, err := netns.NetemStart(req)
	if err != nil {
		return err
	}
	if req.Sidecar.Image == "" {
		return netns.NetemApplyError(argv, c.execFaultInContainer(c.nsCtx(ctx), req.Container.ID(), argv[0], argv[1:]))
	}
	labels := ctr.SidecarLabels(req.RunID, req.Container.ID(), ctr.SidecarKindNetem)
	return c.withSidecar(ctx, req.Container, req.Sidecar, labels, func(ctx context.Context, task containerd.Task) error {
		// Classify execution before withSidecar adds helper-removal failures.
		_, _, err := c.runSidecarCommand(ctx, task, argv[0], argv[1:], execFaultTaskOutput)
		return netns.NetemApplyError(argv, err)
	})
}

// StopNetemContainer removes only a verified Pumba-owned netem topology.
func (c *containerdClient) StopNetemContainer(ctx context.Context, req *ctr.NetemRequest) error {
	log.WithFields(log.Fields{"id": req.Container.ID(), "interface": req.Interface, "tc-image": req.Sidecar.Image}).Debug("stop netem on containerd container")
	if req.DryRun {
		return nil
	}
	argv, err := netns.NetemStop(req)
	if err != nil {
		return err
	}
	return c.runNetemCommand(ctx, req, argv)
}

// PlanNetemContainer returns the argv NetemContainer and StopNetemContainer
// run for req.
func (c *containerdClient) PlanNetemContainer(req *ctr.NetemRequest) (*ctr.CommandPlan, error) {
	return netns.NetemPlan(req)
}

func (c *containerdClient) runNetemCommand(ctx context.Context, req *ctr.NetemRequest, argv []string) error {
	if req.Sidecar.Image != "" {
		labels := ctr.SidecarLabels(req.RunID, req.Container.ID(), ctr.SidecarKindNetem)
		return c.sidecarExec(ctx, req.Container, req.Sidecar, labels, [][]string{argv})
	}
	return c.execFaultInContainer(c.nsCtx(ctx), req.Container.ID(), argv[0], argv[1:])
}
