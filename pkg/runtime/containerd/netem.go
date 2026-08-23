package containerd

import (
	"context"

	ctr "github.com/alexei-led/pumba/pkg/container"
	tcplan "github.com/alexei-led/pumba/pkg/tc"
	log "github.com/sirupsen/logrus"
)

// NetemContainer applies ownership-safe network emulation to a container.
func (c *containerdClient) NetemContainer(ctx context.Context, req *ctr.NetemRequest) error {
	log.WithFields(log.Fields{"id": req.Container.ID(), "interface": req.Interface, "tc-image": req.Sidecar.Image}).Debug("netem on containerd container")
	if req.DryRun {
		return nil
	}
	return c.runNetemPlan(ctx, req, tcplan.Start(containerdNetemPlanRequest(req)))
}

// StopNetemContainer removes only a verified Pumba-owned netem topology.
func (c *containerdClient) StopNetemContainer(ctx context.Context, req *ctr.NetemRequest) error {
	log.WithFields(log.Fields{"id": req.Container.ID(), "interface": req.Interface, "tc-image": req.Sidecar.Image}).Debug("stop netem on containerd container")
	if req.DryRun {
		return nil
	}
	return c.runNetemPlan(ctx, req, tcplan.Stop(containerdNetemPlanRequest(req)))
}

func (c *containerdClient) runNetemPlan(ctx context.Context, req *ctr.NetemRequest, args []string) error {
	if req.Sidecar.Image != "" {
		return c.sidecarExec(ctx, req.Container, req.Sidecar.Image, req.Sidecar.Pull, "sh", [][]string{args})
	}
	return c.execInContainer(c.nsCtx(ctx), req.Container.ID(), "sh", args)
}

func containerdNetemPlanRequest(req *ctr.NetemRequest) *tcplan.NetemRequest {
	ips := make([]string, 0, len(req.IPs))
	for _, ip := range req.IPs {
		ips = append(ips, ip.String())
	}
	return &tcplan.NetemRequest{
		Interface: req.Interface,
		Command:   req.Command,
		IPs:       ips,
		SPorts:    req.SPorts,
		DPorts:    req.DPorts,
	}
}
