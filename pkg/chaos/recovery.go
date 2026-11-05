package chaos

import (
	"time"

	"github.com/alexei-led/pumba/pkg/container"
)

// ArtifactResult reports orphan recovery without inventing a target result.
// The pinned identity is also the complete dry-run sidecar-removal plan.
type ArtifactResult struct {
	Artifact container.SidecarIdentity `json:"artifact"`
	Started  time.Time                 `json:"started"`
	Ended    time.Time                 `json:"ended"`
	Status   TargetStatus              `json:"status"`
	Error    *EventError               `json:"error"`
}

// RecoveryPlan is the typed target recovery plan, including exact removals.
type RecoveryPlan struct {
	Rules      *Plan                       `json:"rules"`
	Sidecars   []container.SidecarIdentity `json:"sidecars"`
	Unresolved []string                    `json:"unresolved"`
}

// SetRecovery attaches recovery's plan and coverage to a real target result.
func (t *Target) SetRecovery(plan *container.CommandPlan, sidecars []container.SidecarIdentity, unresolved []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	var rules *Plan
	if plan != nil {
		rules = &Plan{Via: string(plan.Via), Image: plan.Image, Apply: plan.Apply, Revert: plan.Revert}
	}
	t.recovery = &RecoveryPlan{Rules: rules, Sidecars: sidecars, Unresolved: unresolved}
}
