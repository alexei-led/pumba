package main

import (
	"fmt"
	"io"

	"github.com/alexei-led/pumba/pkg/chaos"
)

// recoveryText renders typed recovery results without implying global
// cleanliness. NDJSON uses the same events without this text subscriber.
type recoveryText struct{ w io.Writer }

func (r recoveryText) Emit(ev *chaos.Event) {
	if ev.Action != "revert" {
		return
	}
	if ev.Artifact != nil {
		a := ev.Artifact
		_, _ = fmt.Fprintf(r.w, "%s orphan %s sidecar %s (original target %s)\n", a.Status, a.Artifact.Kind, a.Artifact.ID, a.Artifact.TargetID)
		if a.Error != nil {
			_, _ = fmt.Fprintln(r.w, a.Error.Message)
		}
	}
	if ev.Result == nil || ev.Result.Recovery == nil {
		return
	}
	result, p := ev.Result, ev.Result.Recovery
	_, _ = fmt.Fprintf(r.w, "%s recovery on %s\n", result.Status, result.Target.Name)
	if result.Cleanup == nil && p.Rules != nil {
		_ = renderPlan(r.w, result.Target, p.Rules)
	}
	for _, a := range p.Sidecars {
		_, _ = fmt.Fprintf(r.w, "  %s sidecar %s (%s)\n", result.Status, a.ID, a.Kind)
	}
	for _, reason := range p.Unresolved {
		_, _ = fmt.Fprintf(r.w, "  unresolved: %s\n", reason)
	}
	if result.Error != nil {
		_, _ = fmt.Fprintln(r.w, result.Error.Message)
	}
}
