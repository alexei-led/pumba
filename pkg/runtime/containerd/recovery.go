package containerd

import (
	"context"
	"errors"
	"fmt"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/errdefs"
)

const recoveryTimeout = 30 * time.Second

// PlanRecoveryRules validates and renders a pure recovery plan.
func (c *containerdClient) PlanRecoveryRules(req *ctr.RuleRecoveryRequest) (*ctr.CommandPlan, error) {
	return netns.RecoveryPlan(req)
}

// RecoverRules removes only validated, freshly observed snapshot occurrences.
func (c *containerdClient) RecoverRules(ctx context.Context, req *ctr.RuleRecoveryRequest) error {
	if _, err := netns.RecoveryPlan(req); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx = c.nsCtx(ctx)
	if req.Sidecar.Image == "" {
		return netns.RecoverRules(ctx, req, func(op context.Context, argv []string) (string, string, error) {
			return c.execContainerCommand(op, req.Container.ID(), argv[0], argv[1:], execTaskOutput)
		})
	}
	labels := ctr.SidecarLabels(req.InvocationID, req.Container.ID(), ctr.SidecarKindInspect)
	return c.withSidecar(ctx, req.Container, req.Sidecar, labels, func(op context.Context, task containerd.Task) error {
		return netns.RecoverRules(op, req, func(execCtx context.Context, argv []string) (string, string, error) {
			return c.runSidecarCommand(execCtx, task, argv[0], argv[1:], execTaskOutput)
		})
	})
}

// RemoveRecoverySidecar pins the loaded container, revalidates its labels,
// kills its task PID 1 (and thereby its PID namespace), removes only its
// snapshot, and verifies absence. PID-namespace death also kills inject-cgroup
// workers that moved out of the original task cgroup.
func (c *containerdClient) RemoveRecoverySidecar(ctx context.Context, identity ctr.SidecarIdentity) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx = c.nsCtx(ctx)
	cntr, err := c.client.LoadContainer(ctx, identity.ID)
	if errdefs.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	info, err := cntr.Info(ctx)
	if err != nil {
		return err
	}
	if err := identity.Matches(cntr.ID(), info.Labels); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), recoveryTimeout)
	defer cancel()
	removeErr := c.cleanupSidecar(cleanup, cntr)
	_, err = c.client.LoadContainer(cleanup, identity.ID)
	if errdefs.IsNotFound(err) {
		return removeErr
	}
	if err != nil {
		return errors.Join(removeErr, fmt.Errorf("verify recovery sidecar: %w", err))
	}
	return errors.Join(removeErr, fmt.Errorf("recovery sidecar %s still exists", identity.ID))
}
