// Package status implements `pumba status`: it reports the Pumba faults
// (tagged iptables rules, Pumba qdiscs) and sidecars left on containers, and
// with --check turns them into a clean/dirty/unknown verdict.
package status

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// Params are the status command parameters.
type Params struct {
	// RunFilter limits rules and sidecars to one run; empty means any run.
	// Qdiscs carry no run and are never filtered.
	RunFilter string
	// Check makes the verdict the run outcome.
	Check bool
	// Sidecar is the image hint for the short-lived inspect sidecar.
	Sidecar container.SidecarSpec
}

// ErrCheckWithInterval rejects --check with --interval: a verdict answers once.
var ErrCheckWithInterval = errors.New("--check cannot be combined with --interval")

// statusClient is the narrow interface the status command needs.
type statusClient interface {
	container.Lister
	container.Inspector
}

type statusCommand struct {
	client statusClient
	gp     *chaos.GlobalParams
	p      Params
}

// NewCommand returns the status command. It only reads: --random and
// --dry-run do not apply, and gp is set so events report them off.
func NewCommand(client statusClient, gp *chaos.GlobalParams, p Params) chaos.Command {
	gp.Random, gp.DryRun = false, false
	return &statusCommand{client: client, gp: gp, p: p}
}

// Run inspects every selected running target once (one tick): it lists Pumba
// sidecars, inspects each target's netns through chaos.RunOnContainers, and
// records the report with the verdict. An inspection failure fails its target
// and the run. A listing failure records an unknown report, so --check never
// passes on a run that saw nothing; a refusal or usage error records none.
func (s *statusCommand) Run(ctx context.Context, _ bool) error {
	sidecars, gaps, err := s.listSidecars(ctx)
	if err != nil {
		err = chaos.NewRuntimeUnavailableError(fmt.Errorf("listing pumba sidecars: %w", err))
		s.recordListingFailure(err)
		return err
	}
	byTarget := map[string][]chaos.StateSidecar{}
	for _, sc := range sidecars {
		byTarget[sc.TargetID] = append(byTarget[sc.TargetID], sc)
	}
	var (
		mu      sync.Mutex
		targets []chaos.StatusTarget
	)
	runErr := chaos.RunOnContainers(ctx, s.client, s.gp, 0, false, true,
		func(ctx context.Context, c *container.Container, t *chaos.Target) error {
			st, err := s.inspect(ctx, c, byTarget[c.ID()])
			t.SetState(st.State)
			mu.Lock()
			targets = append(targets, st)
			mu.Unlock()
			return err
		})
	if runErr != nil && len(targets) == 0 {
		err := fmt.Errorf("selecting targets: %w", runErr)
		if !errors.Is(runErr, chaos.ErrMaxTargetsExceeded) && !errors.Is(runErr, chaos.ErrNegativeMaxTargets) {
			s.recordListingFailure(err)
		}
		return err
	}
	slices.SortFunc(targets, func(a, b chaos.StatusTarget) int { return strings.Compare(a.Target.Name, b.Target.Name) })
	cov := s.coverage(targets, sidecars, gaps)
	cov.Verdict, cov.Reasons = evaluate(targets, cov)
	s.gp.RecordStatus(&chaos.StatusReport{Check: s.p.Check, Targets: targets, Coverage: cov})
	if runErr != nil {
		return fmt.Errorf("inspecting targets: %w", runErr)
	}
	return nil
}

// recordListingFailure records a report with an unknown verdict for a run that
// could not list containers and so inspected nothing.
func (s *statusCommand) recordListingFailure(err error) {
	s.gp.RecordStatus(&chaos.StatusReport{Check: s.p.Check, Targets: []chaos.StatusTarget{}, Coverage: &chaos.StatusCoverage{
		RunFilter:      s.p.RunFilter,
		OrphanSidecars: []chaos.StateSidecar{},
		MissingTargets: []string{},
		ListingGaps:    []chaos.ListingGap{},
		Verdict:        chaos.VerdictUnknown,
		Reasons:        []string{"listing failed: " + err.Error()},
	}})
}

// listSidecars lists every Pumba sidecar except inspect sidecars, running or
// not, matching the run filter. It uses a filter func rather than
// ListOpts.Labels, which containerd rejects.
func (s *statusCommand) listSidecars(ctx context.Context) ([]chaos.StateSidecar, []chaos.ListingGap, error) {
	var (
		mu   sync.Mutex
		gaps = []chaos.ListingGap{}
	)
	opts := container.ListOpts{All: true, OnSkip: func(id string, err error) {
		mu.Lock()
		defer mu.Unlock()
		gaps = append(gaps, chaos.ListingGap{ID: id, Error: err.Error()})
	}}
	found, err := s.client.ListContainers(ctx, func(c *container.Container) bool {
		kind := c.Labels[container.KindLabel]
		return kind != "" && kind != container.SidecarKindInspect && s.matchesRun(c.Labels[container.RunIDLabel])
	}, opts)
	if err != nil {
		return nil, nil, err
	}
	sidecars := make([]chaos.StateSidecar, 0, len(found))
	for _, c := range found {
		sidecars = append(sidecars, chaos.StateSidecar{
			ID:       c.ID(),
			Name:     c.Name(),
			Kind:     c.Labels[container.KindLabel],
			State:    c.State,
			TargetID: c.Labels[container.TargetLabel],
			RunID:    optional(c.Labels[container.RunIDLabel]),
		})
	}
	return sidecars, gaps, nil
}

// inspect reads one target's netns. The state always carries the target's
// sidecars, also when inspection failed.
func (s *statusCommand) inspect(ctx context.Context, c *container.Container, sidecars []chaos.StateSidecar) (chaos.StatusTarget, error) {
	st := chaos.StatusTarget{
		Target: chaos.TargetRef{ID: c.ID(), Name: c.Name()},
		State:  &chaos.TargetState{Rules: []chaos.StateRule{}, Qdiscs: []chaos.StateQdisc{}, Sidecars: nonNil(sidecars)},
	}
	fs, err := s.client.InspectNetns(ctx, &container.InspectRequest{Container: c, Sidecar: s.p.Sidecar, RunID: s.gp.RunID})
	if err != nil {
		err = chaos.NewInspectError(fmt.Errorf("inspecting %s: %w", c.Name(), err))
		log.WithError(err).WithField("container", c.Name()).Debug("status inspection failed")
		st.Error = err.Error()
		return st, err
	}
	for _, r := range fs.Rules {
		if s.matchesRun(r.RunID) {
			st.State.Rules = append(st.State.Rules, chaos.StateRule{RunID: r.RunID, Chain: r.Chain, Spec: r.Spec})
		}
	}
	for _, q := range fs.Qdiscs {
		st.State.Qdiscs = append(st.State.Qdiscs, chaos.StateQdisc{Device: q.Device, Kind: q.Kind, Ownership: string(q.Ownership)})
	}
	return st, nil
}

// coverage collects findings outside the inspected targets: orphan sidecars
// (only without a selection), named targets that were not inspected, and
// listing gaps.
func (s *statusCommand) coverage(targets []chaos.StatusTarget, sidecars []chaos.StateSidecar, gaps []chaos.ListingGap) *chaos.StatusCoverage {
	cov := &chaos.StatusCoverage{
		RunFilter:      s.p.RunFilter,
		OrphanSidecars: []chaos.StateSidecar{},
		MissingTargets: []string{},
		ListingGaps:    gaps,
	}
	inspected := make(map[string]bool, len(targets))
	for _, t := range targets {
		inspected[t.Target.ID] = true
	}
	if len(s.gp.Names) == 0 && s.gp.Pattern == "" && len(s.gp.Labels) == 0 {
		for _, sc := range sidecars {
			if !inspected[sc.TargetID] {
				cov.OrphanSidecars = append(cov.OrphanSidecars, sc)
			}
		}
	}
	for _, name := range s.gp.Names {
		if !slices.ContainsFunc(targets, func(t chaos.StatusTarget) bool { return named(name, t.Target) }) {
			cov.MissingTargets = append(cov.MissingTargets, name)
		}
	}
	return cov
}

func (s *statusCommand) matchesRun(runID string) bool {
	return s.p.RunFilter == "" || runID == s.p.RunFilter
}

// named matches a container name argument the way target selection does:
// by name, with or without Docker's leading slash, or by ID.
func named(name string, t chaos.TargetRef) bool {
	return name == t.ID || name == t.Name || name == strings.TrimPrefix(t.Name, "/")
}

// evaluate decides the verdict and its reasons. Dirty wins over unknown: a
// matched attributable artifact is certain even when coverage is incomplete.
// Clean is never vacuous: no inspected target is unknown.
func evaluate(targets []chaos.StatusTarget, cov *chaos.StatusCoverage) (chaos.Verdict, []string) {
	var dirty, unknown []string
	runScoped := cov.RunFilter != ""
	for _, t := range targets {
		d, u := evaluateTarget(t, runScoped)
		dirty, unknown = append(dirty, d...), append(unknown, u...)
	}
	for _, sc := range cov.OrphanSidecars {
		dirty = append(dirty, fmt.Sprintf("orphan %s sidecar %s (target %s)", sc.Kind, sc.Name, sc.TargetID))
	}
	for _, name := range cov.MissingTargets {
		unknown = append(unknown, fmt.Sprintf("%s: not found or not running", name))
	}
	for _, g := range cov.ListingGaps {
		unknown = append(unknown, fmt.Sprintf("%s: container could not be read", g.ID))
	}
	if len(targets) == 0 {
		unknown = append(unknown, "no targets selected")
	}
	switch {
	case len(dirty) > 0:
		return chaos.VerdictDirty, dirty
	case len(unknown) > 0:
		return chaos.VerdictUnknown, unknown
	default:
		return chaos.VerdictClean, []string{}
	}
}

func evaluateTarget(t chaos.StatusTarget, runScoped bool) (dirty, unknown []string) {
	name := t.Target.Name
	if t.Error != "" {
		unknown = append(unknown, name+": inspection failed")
	}
	if t.State == nil {
		return dirty, unknown
	}
	for _, r := range t.State.Rules {
		dirty = append(dirty, fmt.Sprintf("%s: iptables rule of run %s", name, r.RunID))
	}
	for _, sc := range t.State.Sidecars {
		dirty = append(dirty, fmt.Sprintf("%s: %s sidecar %s", name, sc.Kind, sc.Name))
	}
	for _, q := range t.State.Qdiscs {
		switch {
		case q.Ownership != string(container.QdiscPumbaOwned):
			unknown = append(unknown, fmt.Sprintf("%s: %s qdisc on %s", name, q.Ownership, q.Device))
		case runScoped:
			unknown = append(unknown, fmt.Sprintf("%s: pumba qdisc on %s not attributable to a run", name, q.Device))
		default:
			dirty = append(dirty, fmt.Sprintf("%s: pumba qdisc on %s", name, q.Device))
		}
	}
	return dirty, unknown
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
