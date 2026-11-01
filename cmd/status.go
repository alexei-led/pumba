package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/alexei-led/pumba/pkg/chaos"
	log "github.com/sirupsen/logrus"
)

// tabwriter layout of the status table.
const (
	statusMinWidth = 0
	statusTabWidth = 8
	statusPadding  = 2
)

// RecordStatus hands a status report to every subscriber that takes them.
func (f *eventFanout) RecordStatus(rep *chaos.StatusReport) {
	for _, s := range f.subs {
		if r, ok := s.(chaos.StatusRecorder); ok {
			r.RecordStatus(rep)
		}
	}
}

// statusText prints each `pumba status` report to stdout in text mode.
type statusText struct {
	w io.Writer
}

// Emit ignores events: text mode prints only status reports.
func (statusText) Emit(*chaos.Event) {}

func (s statusText) RecordStatus(rep *chaos.StatusReport) {
	if err := renderStatus(s.w, rep); err != nil {
		log.WithError(err).Warn("writing status")
	}
}

// renderStatus writes a status report as a TARGET KIND DETAIL RUN table, plus
// the verdict line when the report decides the outcome (--check).
func renderStatus(w io.Writer, rep *chaos.StatusReport) error {
	tw := tabwriter.NewWriter(w, statusMinWidth, statusTabWidth, statusPadding, ' ', 0)
	row := func(cols ...string) { _, _ = fmt.Fprintln(tw, strings.Join(cols, "\t")) }
	row("TARGET", "KIND", "DETAIL", "RUN")
	for _, t := range rep.Targets {
		statusTargetRows(row, t)
	}
	cov := rep.Coverage
	for _, sc := range cov.OrphanSidecars {
		row(shortID(sc.TargetID), "sidecar", fmt.Sprintf("orphan %s %s (%s)", sc.Kind, sc.Name, sc.State), runOf(sc.RunID))
	}
	for _, name := range cov.MissingTargets {
		row(name, "missing", "not found or not running", "-")
	}
	for _, g := range cov.ListingGaps {
		row(shortID(g.ID), "unreadable", g.Error, "-")
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("writing status table: %w", err)
	}
	if !rep.Check {
		return nil
	}
	line := "verdict: " + string(cov.Verdict)
	if len(cov.Reasons) > 0 {
		line += " (" + strings.Join(cov.Reasons, "; ") + ")"
	}
	if _, err := fmt.Fprintln(w, line); err != nil {
		return fmt.Errorf("writing status verdict: %w", err)
	}
	return nil
}

func statusTargetRows(row func(...string), t chaos.StatusTarget) {
	name := t.Target.Name
	if t.Error != "" {
		row(name, "error", t.Error, "-")
	}
	if t.State == nil {
		return
	}
	for _, r := range t.State.Rules {
		row(name, "iptables", r.Spec, r.RunID)
	}
	for _, q := range t.State.Qdiscs {
		row(name, "qdisc", fmt.Sprintf("%s %s (%s)", q.Device, q.Kind, q.Ownership), "-")
	}
	for _, sc := range t.State.Sidecars {
		row(name, "sidecar", fmt.Sprintf("%s %s (%s)", sc.Kind, sc.Name, sc.State), runOf(sc.RunID))
	}
	if t.Error == "" && len(t.State.Rules)+len(t.State.Qdiscs)+len(t.State.Sidecars) == 0 {
		row(name, "-", "clean", "-")
	}
}

func runOf(id *string) string {
	if id == nil {
		return "-"
	}
	return *id
}

// shortIDLen is the usual short container ID length.
const shortIDLen = 12

func shortID(id string) string {
	if len(id) > shortIDLen {
		return id[:shortIDLen]
	}
	return id
}
