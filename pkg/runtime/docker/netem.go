package docker

import (
	"context"

	ctr "github.com/alexei-led/pumba/pkg/container"
	tcplan "github.com/alexei-led/pumba/pkg/tc"
	log "github.com/sirupsen/logrus"
)

// NetemContainer injects netem into the given container network namespace.
// The shared tc planner probes qdisc ownership before it mutates the device.
func (client dockerClient) NetemContainer(ctx context.Context, req *ctr.NetemRequest) error {
	log.WithFields(log.Fields{
		"name": req.Container.Name(), "id": req.Container.ID(), "command": req.Command,
		"ips": req.IPs, "sports": req.SPorts, "dports": req.DPorts, "dryrun": req.DryRun,
	}).Info("running netem on container")
	if req.DryRun {
		return nil
	}
	return client.runNetemPlan(ctx, req, tcplan.Start(netemPlanRequest(req)))
}

// StopNetemContainer removes only a verified Pumba-owned netem topology.
func (client dockerClient) StopNetemContainer(ctx context.Context, req *ctr.NetemRequest) error {
	log.WithFields(log.Fields{
		"name": req.Container.Name(), "id": req.Container.ID(), "ips": req.IPs,
		"sports": req.SPorts, "dports": req.DPorts, "dryrun": req.DryRun,
	}).Info("stopping netem on container")
	if req.DryRun {
		return nil
	}
	return client.runNetemPlan(ctx, req, tcplan.Stop(netemPlanRequest(req)))
}

func (client dockerClient) runNetemPlan(ctx context.Context, req *ctr.NetemRequest, args []string) error {
	if req.Sidecar.Image == "" {
		return client.execOnContainer(ctx, req.Container, "sh", args, true)
	}
	return client.runSidecar(ctx, req.Container, [][]string{args}, req.Sidecar.Image, "sh", req.Sidecar.Pull)
}

func netemPlanRequest(req *ctr.NetemRequest) *tcplan.NetemRequest {
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
