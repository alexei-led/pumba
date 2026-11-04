package netns

import (
	"context"
	"errors"
	"fmt"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/tc"
)

const (
	maxRecoveryRules = 256
	maxRecoveryBytes = 64 * 1024
	recoveryTimeout  = 30 * time.Second
)

// RecoveryPlan validates the complete snapshot before any mutation. Each
// occurrence is retained, including identical rules in independent backends.
func RecoveryPlan(req *ctr.RuleRecoveryRequest) (*ctr.CommandPlan, error) {
	if req == nil || req.Container == nil || req.RunFilter == "" {
		return nil, errors.New("incomplete recovery request")
	}
	if len(req.Rules) > maxRecoveryRules {
		return nil, fmt.Errorf("recovery exceeds %d rule occurrences", maxRecoveryRules)
	}
	commands := make([][]string, 0, len(req.Rules))
	size := 0
	for _, r := range req.Rules {
		if r.RunID != req.RunFilter {
			return nil, errors.New("recovery rule outside run filter")
		}
		if len(r.Spec) > maxRecoveryBytes-size {
			return nil, fmt.Errorf("recovery plan exceeds %d bytes", maxRecoveryBytes)
		}
		argv, err := tc.RecoveryDeleteCommand(r.Backend, req.RunFilter, r.Chain, r.Spec)
		if err != nil {
			return nil, err
		}
		for _, arg := range argv {
			if len(arg) >= maxRecoveryBytes-size {
				return nil, fmt.Errorf("recovery plan exceeds %d bytes", maxRecoveryBytes)
			}
			size += len(arg) + 1
		}
		commands = append(commands, argv)
	}
	return newPlan(req.Sidecar, nil, commands), nil
}

// ContextRunner executes literal argv with the supplied operation context.
type ContextRunner func(context.Context, []string) (string, string, error)

// RecoverRules freshly validates a snapshot, deletes at most its occurrence
// count, and verifies absence. Shared namespaces may already be clean. New or
// changed rules never expand the authorization. Cancellation admits no new
// delete; an admitted delete and its verification have one detached deadline.
func RecoverRules(ctx context.Context, req *ctr.RuleRecoveryRequest, run ContextRunner) error {
	plan, err := RecoveryPlan(req)
	if err != nil {
		return err
	}
	inspect := func(ctx context.Context) (*ctr.FaultState, error) {
		return Inspect(func(argv []string) (string, string, error) { return run(ctx, argv) })
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	state, err := inspect(ctx)
	if err != nil {
		return err
	}
	remaining := recoveryCounts(state, req.RunFilter)
	authorized := map[ctr.PumbaRule]int{}
	for _, r := range req.Rules {
		authorized[r]++
	}
	for r, count := range remaining {
		if count > authorized[r] {
			return errors.New("recovery inventory changed; refusing new rule occurrences")
		}
	}
	for i, r := range req.Rules {
		if remaining[r] == 0 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		next, err := deleteAndVerify(ctx, plan.Revert[i], r, remaining[r], inspect, run)
		if err != nil {
			return err
		}
		remaining = recoveryCounts(next, req.RunFilter)
	}
	for _, count := range remaining {
		if count > 0 {
			return errors.New("recovery verification found remaining rules")
		}
	}
	return nil
}

func deleteAndVerify(ctx context.Context, argv []string, rule ctr.PumbaRule, before int,
	inspect func(context.Context) (*ctr.FaultState, error), run ContextRunner,
) (*ctr.FaultState, error) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), recoveryTimeout)
	defer cancel()
	_, _, deleteErr := run(cleanup, argv)
	state, inspectErr := inspect(cleanup)
	if inspectErr != nil {
		return nil, errors.Join(deleteErr, fmt.Errorf("recovery verification: %w", inspectErr))
	}
	// A concurrent cleanup may have removed the rule before our delete. Only
	// verified disappearance, not a backend error string, proves success.
	if recoveryCounts(state, rule.RunID)[rule] >= before {
		return nil, errors.Join(deleteErr, errors.New("recovery rule still present after delete"))
	}
	return state, nil
}

func recoveryCounts(state *ctr.FaultState, runID string) map[ctr.PumbaRule]int {
	counts := map[ctr.PumbaRule]int{}
	for _, r := range state.Rules {
		if r.RunID == runID {
			counts[r]++
		}
	}
	return counts
}
