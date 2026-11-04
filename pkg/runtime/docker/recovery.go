package docker

import (
	"context"
	"errors"
	"fmt"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
	"github.com/containerd/errdefs"
	ctypes "github.com/docker/docker/api/types/container"
)

const recoveryTimeout = 30 * time.Second

// PlanRecoveryRules validates and renders a pure recovery plan.
func (client dockerClient) PlanRecoveryRules(req *ctr.RuleRecoveryRequest) (*ctr.CommandPlan, error) {
	return netns.RecoveryPlan(req)
}

// RecoverRules removes only validated, freshly observed snapshot occurrences.
func (client dockerClient) RecoverRules(ctx context.Context, req *ctr.RuleRecoveryRequest) error {
	if _, err := netns.RecoveryPlan(req); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	recoverIn := func(id string, privileged bool) error {
		return netns.RecoverRules(ctx, req, func(op context.Context, argv []string) (string, string, error) {
			return client.runExecContext(op, id, argv, privileged)
		})
	}
	if req.Sidecar.Image == "" {
		return recoverIn(req.Container.ID(), true)
	}
	labels := ctr.SidecarLabels(req.InvocationID, req.Container.ID(), ctr.SidecarKindInspect)
	return client.withSidecar(ctx, req.Container, req.Sidecar, labels, ctr.SidecarKindInspect, func(id string) error { return recoverIn(id, false) })
}

// RemoveRecoverySidecar rechecks exact identity and ownership immediately
// before force removal, without links or volumes, then verifies absence.
func (client dockerClient) RemoveRecoverySidecar(ctx context.Context, identity ctr.SidecarIdentity) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := client.containerAPI.ContainerInspect(ctx, identity.ID)
	if errdefs.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.ContainerJSONBase == nil || info.Config == nil {
		return errors.New("incomplete sidecar metadata")
	}
	if err := identity.Matches(info.ID, info.Config.Labels); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), recoveryTimeout)
	defer cancel()
	removeErr := client.containerAPI.ContainerRemove(cleanup, identity.ID, ctypes.RemoveOptions{Force: true})
	_, err = client.containerAPI.ContainerInspect(cleanup, identity.ID)
	if errdefs.IsNotFound(err) {
		return nil
	}
	if removeErr != nil {
		return fmt.Errorf("remove recovery sidecar: %w", removeErr)
	}
	if err != nil {
		return fmt.Errorf("verify sidecar absence: %w", err)
	}
	return fmt.Errorf("recovery sidecar %s still exists", identity.ID)
}
