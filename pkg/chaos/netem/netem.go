package netem

import (
	"context"
	"fmt"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// netemClient is the narrow interface needed by all netem commands.
type netemClient interface {
	container.Lister
	container.Netem
}

// cleanupTimeout caps how long the netem-cleanup sidecar cycle is allowed
// to run after abort or scheduled stop. Independent of --duration so a
// 1h chaos run does not give cleanup an hour to complete.
const cleanupTimeout = 30 * time.Second

// run network emulation command, stop netem on timeout or abort
func runNetem(ctx context.Context, client netemClient, req *container.NetemRequest) error {
	logger := log.WithFields(log.Fields{
		"id":       req.Container.ID(),
		"name":     req.Container.Name(),
		"iface":    req.Interface,
		"netem":    req.Command,
		"ips":      req.IPs,
		"sports":   req.SPorts,
		"dports":   req.DPorts,
		"duration": req.Duration,
		"tc-image": req.Sidecar.Image,
		"pull":     req.Sidecar.Pull,
	})
	logger.Debug("running netem command")
	if err := client.NetemContainer(ctx, req); err != nil {
		return fmt.Errorf("netem failed: %w", err)
	}
	logger.Debug("netem command started")

	// create new context with timeout for canceling
	stopCtx, cancel := context.WithTimeout(context.Background(), req.Duration)
	defer cancel()
	// Wait for the requested duration or parent cancellation. Cleanup gets an
	// independent bounded context, then its error is returned to the caller so
	// ownership failures cannot be reported as a successful chaos run.
	select {
	case <-ctx.Done():
		logger.Debug("stopping netem command on abort")
	case <-stopCtx.Done():
		logger.Debug("stopping netem command on timeout")
	}
	if err := stopNetem(ctx, client, req); err != nil {
		return fmt.Errorf("stopping netem: %w", err)
	}
	return nil
}

func stopNetem(ctx context.Context, client netemClient, req *container.NetemRequest) error {
	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cleanupCancel()
	return client.StopNetemContainer(cleanupCtx, req)
}
