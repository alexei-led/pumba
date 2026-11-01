package main

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/alexei-led/pumba/pkg/chaos"
	log "github.com/sirupsen/logrus"
)

// planText prints the plan of every dry-run target.result to stdout in text
// mode, as "would run" lines.
type planText struct {
	mu sync.Mutex // one target's lines stay together under parallel fanout
	w  io.Writer
}

// Emit renders target.result events that carry a plan and ignores the rest.
// It writes synchronously: plans exist only in dry-runs, where no fault is
// active that a slow stdout could hold.
func (p *planText) Emit(ev *chaos.Event) {
	if ev.Type != chaos.EventTargetResult || ev.Result.Plan == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := renderPlan(p.w, ev.Result.Target, ev.Result.Plan); err != nil {
		log.WithError(err).Warn("writing dry-run plan")
	}
}

// renderPlan writes where the commands of plan would run on target, then
// each apply and revert command as a shell-quoted line.
func renderPlan(w io.Writer, target chaos.TargetRef, plan *chaos.Plan) error {
	name := target.Name
	if name == "" {
		name = shortID(target.ID)
	}
	via := "exec in the target"
	if plan.Image != "" {
		via = "sidecar " + plan.Image
	}
	var b strings.Builder
	fmt.Fprintf(&b, "would run on %s via %s:\n", name, via)
	for _, argv := range plan.Apply {
		fmt.Fprintf(&b, "  apply:  %s\n", shellJoin(argv))
	}
	for _, argv := range plan.Revert {
		fmt.Fprintf(&b, "  revert: %s\n", shellJoin(argv))
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("writing dry-run plan: %w", err)
	}
	return nil
}

// shellSafe matches words a POSIX shell reads literally.
var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellJoin quotes argv so the line can be pasted into a shell as is.
func shellJoin(argv []string) string {
	words := make([]string, 0, len(argv))
	for _, arg := range argv {
		if shellSafe.MatchString(arg) {
			words = append(words, arg)
			continue
		}
		words = append(words, "'"+strings.ReplaceAll(arg, "'", `'"'"'`)+"'")
	}
	return strings.Join(words, " ")
}
