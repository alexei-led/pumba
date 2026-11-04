package revert

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
)

// gapLister makes incomplete runtime inventories authorization failures.
type gapLister struct {
	container.Lister
	onSkip func(string, error)
}

func (l gapLister) ListContainers(ctx context.Context, filter container.FilterFunc, opts container.ListOpts) ([]*container.Container, error) {
	opts.OnSkip = l.onSkip
	return l.Lister.ListContainers(ctx, filter, opts)
}

func (c *command) inventory(ctx context.Context) (chaos.PreparedWork, map[string][]container.SidecarIdentity, []string, error) {
	work := chaos.PreparedWork{}
	if err := ctx.Err(); err != nil {
		return work, nil, nil, err
	}
	if c.gp.MaxTargets < 0 {
		return work, nil, nil, chaos.ErrNegativeMaxTargets
	}
	var gaps []error
	var mu sync.Mutex
	lister := gapLister{Lister: c.client, onSkip: func(id string, err error) {
		mu.Lock()
		defer mu.Unlock()
		gaps = append(gaps, fmt.Errorf("inventory gap %s: %w", id, err))
	}}
	targets, targetErr := container.ListNContainers(ctx, lister, c.gp.Names, c.gp.Pattern, c.gp.Labels, 0)
	// Attempt both listings before rejecting an incomplete inventory.
	if err := ctx.Err(); err != nil {
		return work, nil, nil, err
	}
	sidecars, sidecarErr := lister.ListContainers(ctx, c.sidecarFilter, container.ListOpts{All: true})
	if err := errors.Join(targetErr, sidecarErr, errors.Join(gaps...), ctx.Err()); err != nil {
		return work, nil, nil, chaos.NewRuntimeUnavailableError(err)
	}
	return c.selectArtifacts(targets, sidecars)
}

func (c *command) selectArtifacts(targets, sidecars []*container.Container) (chaos.PreparedWork, map[string][]container.SidecarIdentity, []string, error) {
	work := chaos.PreparedWork{Targets: targets}
	byTarget := map[string][]container.SidecarIdentity{}
	selected := map[string]bool{}
	for _, t := range targets {
		selected[t.ID()] = true
	}
	var unresolved []string
	seenSidecars := map[string]bool{}
	for _, sc := range sidecars {
		id := container.SidecarIdentity{ID: sc.ID(), TargetID: sc.Labels[container.TargetLabel], RunID: sc.Labels[container.RunIDLabel], Kind: sc.Labels[container.KindLabel]}
		if selected[sc.ID()] || seenSidecars[sc.ID()] {
			return work, nil, nil, fmt.Errorf("ambiguous sidecar identity %s", sc.ID())
		}
		seenSidecars[sc.ID()] = true
		// Validate before using metadata to exclude an apparent orphan. Missing
		// or invalid attribution can never silently authorize partial recovery.
		if err := id.Matches(sc.ID(), sc.Labels); err != nil {
			return work, nil, nil, err
		}
		if selected[id.TargetID] {
			byTarget[id.TargetID] = append(byTarget[id.TargetID], id)
			continue
		}
		if c.orphanSelected(id) {
			work.Orphans = append(work.Orphans, id)
			unresolved = append(unresolved, fmt.Sprintf("target %s missing, stopped or unselected: namespace coverage unavailable", id.TargetID))
		}
	}
	for _, name := range c.gp.Names {
		if !slices.ContainsFunc(targets, func(t *container.Container) bool {
			return name == t.ID() || name == t.Name() || name == strings.TrimPrefix(t.Name(), "/")
		}) {
			unresolved = append(unresolved, fmt.Sprintf("explicit target %s not found or not running", name))
		}
	}
	return work, byTarget, unresolved, nil
}

func (c *command) sidecarFilter(sc *container.Container) bool {
	return sc.Labels[container.RunIDLabel] == c.p.RunFilter && sc.Labels[container.KindLabel] != container.SidecarKindInspect
}

func (c *command) orphanSelected(id container.SidecarIdentity) bool {
	if len(c.gp.Names) == 0 && c.gp.Pattern == "" && len(c.gp.Labels) == 0 {
		return true
	}
	// An exact full original ID is the only provable orphan selector. A name,
	// prefix, regex or label cannot be reconstructed from sidecar metadata.
	return len(c.gp.Labels) == 0 && slices.Contains(c.gp.Names, id.TargetID)
}

func (c *command) verifySidecars(ctx context.Context, work chaos.PreparedWork) error {
	selected := map[string]bool{}
	for _, t := range work.Targets {
		selected[t.ID()] = true
	}
	var gaps []error
	var mu sync.Mutex
	found, err := c.client.ListContainers(ctx, c.sidecarFilter, container.ListOpts{All: true, OnSkip: func(id string, err error) {
		mu.Lock()
		defer mu.Unlock()
		gaps = append(gaps, fmt.Errorf("verification gap %s: %w", id, err))
	}})
	if err != nil {
		return err
	}
	for _, sc := range found {
		id := container.SidecarIdentity{ID: sc.ID(), TargetID: sc.Labels[container.TargetLabel], RunID: sc.Labels[container.RunIDLabel], Kind: sc.Labels[container.KindLabel]}
		if err := id.Matches(sc.ID(), sc.Labels); err != nil {
			gaps = append(gaps, err)
			continue
		}
		if selected[id.TargetID] || c.orphanSelected(id) {
			gaps = append(gaps, fmt.Errorf("recovery sidecar %s remains or appeared during recovery", id.ID))
		}
	}
	return errors.Join(gaps...)
}
