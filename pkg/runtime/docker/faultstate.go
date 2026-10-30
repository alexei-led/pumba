package docker

import (
	"context"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
	log "github.com/sirupsen/logrus"
)

// InspectNetns reads the Pumba fault state of the target's network namespace
// in a short-lived labeled sidecar or, without a sidecar image, by exec in the
// target. It changes nothing in the netns.
func (client dockerClient) InspectNetns(ctx context.Context, req *ctr.InspectRequest) (*ctr.FaultState, error) {
	log.WithFields(log.Fields{
		"name": req.Container.Name(), "id": req.Container.ID(),
		"img": req.Sidecar.Image, "pull": req.Sidecar.Pull,
	}).Debug("inspecting container network namespace")
	if req.Sidecar.Image == "" {
		return netns.Inspect(func(argv []string) (string, string, error) {
			return client.runExec(ctx, req.Container.ID(), argv, true)
		})
	}
	var state *ctr.FaultState
	labels := ctr.SidecarLabels(req.RunID, req.Container.ID(), ctr.SidecarKindInspect)
	err := client.withSidecar(ctx, req.Container, req.Sidecar, labels, ctr.SidecarKindInspect, func(sidecarID string) error {
		var err error
		state, err = netns.Inspect(func(argv []string) (string, string, error) {
			return client.runExec(ctx, sidecarID, argv, false)
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}
