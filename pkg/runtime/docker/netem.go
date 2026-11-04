package docker

import (
	"context"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
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
	argv, err := netns.NetemStart(req)
	if err != nil {
		return err
	}
	return netns.NetemApplyError(argv, client.runNetemCommand(ctx, req, argv))
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
	argv, err := netns.NetemStop(req)
	if err != nil {
		return err
	}
	return client.runNetemCommand(ctx, req, argv)
}

// PlanNetemContainer returns the argv NetemContainer and StopNetemContainer
// run for req. The `which sh` probe of exec mode is not part of the plan.
func (client dockerClient) PlanNetemContainer(req *ctr.NetemRequest) (*ctr.CommandPlan, error) {
	return netns.NetemPlan(req)
}

func (client dockerClient) runNetemCommand(ctx context.Context, req *ctr.NetemRequest, argv []string) error {
	if req.Sidecar.Image == "" {
		return client.execOnContainer(ctx, req.Container, argv[0], argv[1:], true)
	}
	labels := ctr.SidecarLabels(req.RunID, req.Container.ID(), ctr.SidecarKindNetem)
	return client.runSidecar(ctx, req.Container, req.Sidecar, labels, ctr.SidecarKindNetem, [][]string{argv})
}
