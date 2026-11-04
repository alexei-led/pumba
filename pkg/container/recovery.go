package container

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Recovery exposes only validated, run-scoped recovery operations. Planning is
// pure; execution must freshly inspect and verify absence before returning nil.
type Recovery interface {
	PlanRecoveryRules(*RuleRecoveryRequest) (*CommandPlan, error)
	RecoverRules(context.Context, *RuleRecoveryRequest) error
	RemoveRecoverySidecar(context.Context, SidecarIdentity) error
}

// RuleRecoveryRequest is a bounded inventory snapshot, not arbitrary argv.
// RunFilter authorizes existing rules; InvocationID labels inspection helpers.
type RuleRecoveryRequest struct {
	Container    *Container
	Sidecar      SidecarSpec
	RunFilter    string
	InvocationID string
	Rules        []PumbaRule
}

// SidecarIdentity pins the exact artifact and all ownership labels. It never
// identifies an ordinary target and never authorizes volume removal.
type SidecarIdentity struct {
	ID       string `json:"id"`
	TargetID string `json:"target_id"`
	RunID    string `json:"run_id"`
	Kind     string `json:"kind"`
}

// Validate rejects incomplete or unsafe artifact identities.
func (s SidecarIdentity) Validate() error {
	if s.ID == "" || s.TargetID == "" || s.ID == s.TargetID || s.RunID == "" || strings.ContainsAny(s.ID+s.TargetID, " \t\r\n/") {
		return errors.New("invalid recovery sidecar identity")
	}
	switch s.Kind {
	case SidecarKindNetem, SidecarKindIPTables, SidecarKindStress:
		return nil
	default:
		return fmt.Errorf("ineligible recovery sidecar kind %q", s.Kind)
	}
}

// Matches checks fresh metadata immediately before removal. Labels alone do
// not prove the original writer stopped; the operator must stop it first.
func (s SidecarIdentity) Matches(id string, labels map[string]string) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if id != s.ID || labels[SkipLabel] != trueValue || labels[TargetLabel] != s.TargetID || labels[RunIDLabel] != s.RunID || labels[KindLabel] != s.Kind {
		return fmt.Errorf("recovery sidecar %s identity or ownership changed", s.ID)
	}
	return nil
}
