package podman

import (
	"context"

	ctr "github.com/alexei-led/pumba/pkg/container"
)

// PlanRecoveryRules applies the same rootless guard as netns inspection.
func (p *podmanClient) PlanRecoveryRules(req *ctr.RuleRecoveryRequest) (*ctr.CommandPlan, error) {
	if p.rootless {
		return nil, rootlessError("revert", p.socketURI)
	}
	return p.Client.PlanRecoveryRules(req)
}

// RecoverRules applies the same rootless guard as netns inspection. Sidecar
// removal inherits Docker's exact-ID/label check and volume-free removal.
func (p *podmanClient) RecoverRules(ctx context.Context, req *ctr.RuleRecoveryRequest) error {
	if p.rootless {
		return rootlessError("revert", p.socketURI)
	}
	return p.Client.RecoverRules(ctx, req)
}
