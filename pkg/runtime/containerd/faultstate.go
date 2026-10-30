package containerd

import (
	"context"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
	containerd "github.com/containerd/containerd/v2/client"
	log "github.com/sirupsen/logrus"
)

// InspectNetns reads the Pumba fault state of the target's network namespace
// in a short-lived labeled sidecar or, without a sidecar image, by exec in the
// target. It changes nothing in the netns.
func (c *containerdClient) InspectNetns(ctx context.Context, req *ctr.InspectRequest) (*ctr.FaultState, error) {
	log.WithFields(log.Fields{"id": req.Container.ID(), "image": req.Sidecar.Image}).Debug("inspecting containerd container network namespace")
	if req.Sidecar.Image == "" {
		nsCtx := c.nsCtx(ctx)
		return netns.Inspect(func(argv []string) (string, error) {
			return c.execInContainerOutput(nsCtx, req.Container.ID(), argv[0], argv[1:])
		})
	}
	var state *ctr.FaultState
	labels := ctr.SidecarLabels(req.RunID, req.Container.ID(), ctr.SidecarKindInspect)
	err := c.withSidecar(ctx, req.Container, req.Sidecar, labels, func(ctx context.Context, task containerd.Task) error {
		var err error
		state, err = netns.Inspect(func(argv []string) (string, error) {
			return c.runSidecarCmd(ctx, task, argv[0], argv[1:])
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}
