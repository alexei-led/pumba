package netem

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// netemClient is the narrow interface needed by all netem commands.
type netemClient interface {
	container.Lister
	container.AddressResolver
	container.Netem
}

// targetReport is the part of *chaos.Target runNetem reports through.
type targetReport interface {
	Applied()
	Reverted(err error)
	SetPlan(p *container.CommandPlan)
}

// cleanupTimeout caps how long the netem-cleanup sidecar cycle is allowed
// to run after abort or scheduled stop. Independent of --duration so a
// 1h chaos run does not give cleanup an hour to complete.
const cleanupTimeout = 30 * time.Second

// run network emulation command, stop netem on timeout or abort. Reports to t
// when the qdisc is installed and how its removal went; a failed install is
// reported by the returned error alone. A dry-run runs nothing: it reports the
// runtime's plan of the commands to t and returns at once.
func runNetem(ctx context.Context, client netemClient, req *container.NetemRequest, t targetReport) error {
	// Each per-action Run already resolves --target container names/IDs
	// once, before matched containers are enumerated (see e.g. delay.go).
	// This call is a no-op then, since TargetNames is already empty; it
	// only does real work if runNetem is ever called directly with an
	// unresolved request, so req.IPs is always complete by the time it's
	// handed to the runtime client below.
	if err := resolveTargetNames(ctx, client, req); err != nil {
		return fmt.Errorf("failed to resolve --target: %w", err)
	}

	if req.MatchAll && !req.DryRun {
		if _, err := client.PlanNetemContainer(req); err != nil {
			return fmt.Errorf("planning netem: %w", err)
		}
	}
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
	if req.DryRun {
		return planNetem(client, req, t, logger)
	}
	logger.Debug("running netem command")
	if err := client.NetemContainer(ctx, req); err != nil {
		// A proven pre-mutation refusal must not remove a sibling's active
		// fault in a shared namespace. Unknown/partial failures still roll back.
		if errors.Is(err, container.ErrNetemNotApplied) {
			return fmt.Errorf("netem failed: %w", err)
		}
		// Best-effort rollback is not reported as a successful apply/revert.
		if cleanupErr := stopNetem(ctx, client, req); cleanupErr != nil {
			return errors.Join(fmt.Errorf("netem failed: %w", err), fmt.Errorf("stopping netem after failure: %w", cleanupErr))
		}
		return fmt.Errorf("netem failed: %w", err)
	}
	logger.Debug("netem command started")
	t.Applied()
	waitNetem(ctx, req.Duration, logger)
	err := stopNetem(ctx, client, req)
	t.Reverted(err)
	if err != nil {
		return fmt.Errorf("stopping netem: %w", err)
	}
	return nil
}

// planNetem reports the commands a netem run would execute on the target.
func planNetem(client netemClient, req *container.NetemRequest, t targetReport, logger *log.Entry) error {
	plan, err := client.PlanNetemContainer(req)
	if err != nil {
		return fmt.Errorf("planning netem: %w", err)
	}
	logger.WithFields(log.Fields{"via": plan.Via, "apply": plan.Apply, "revert": plan.Revert}).Info("dry-run: planned netem commands")
	t.SetPlan(plan)
	return nil
}

// waitNetem blocks until the requested duration elapses or ctx is cancelled.
// It does no cleanup; the caller stops netem afterwards on a detached context.
func waitNetem(ctx context.Context, duration time.Duration, logger *log.Entry) {
	// create new context with timeout for canceling
	stopCtx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	select {
	case <-ctx.Done():
		logger.Debug("stopping netem command on abort")
	case <-stopCtx.Done():
		logger.Debug("stopping netem command on timeout")
	}
}

func stopNetem(ctx context.Context, client netemClient, req *container.NetemRequest) error {
	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cleanupCancel()
	return client.StopNetemContainer(cleanupCtx, req)
}
