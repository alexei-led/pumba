// Package cliflags wraps urfave/cli flag access behind a thin interface so
// per-command parse functions depend on a portable Flags abstraction rather
// than a specific cli library version. The eventual urfave/cli v3 migration
// becomes a one-file swap (add a V3 adapter, change the wiring in cmd/main.go
// and pkg/chaos/cmd.NewAction).
package cliflags

import "time"

// Flags is the minimum surface every per-command parser needs from the
// underlying CLI library. It deliberately omits flag declarations and parsing
// — those still live with cli.Flag values — and exposes only value lookup.
//
// Methods mirror urfave/cli v1's Context names so the V1 adapter is a 1:1
// pass-through. New CLI library versions plug in by writing another adapter.
type Flags interface {
	String(name string) string
	Bool(name string) bool
	BoolT(name string) bool
	Duration(name string) time.Duration
	Int(name string) int
	Float64(name string) float64
	StringSlice(name string) []string
	Args() []string
	// Parent returns the parent subcommand's flags, or nil at the root.
	// Used by netem/iptables subcommand parsers to read flags declared on
	// the parent action (e.g. delay reads --duration from netem).
	Parent() Flags
	// Global returns the application-level flags (the root context).
	Global() Flags
}
