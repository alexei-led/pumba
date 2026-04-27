package cliflags

import (
	"time"

	"github.com/urfave/cli"
)

// V1 adapts urfave/cli v1's *cli.Context to the Flags interface. The Context
// pointer is exposed so call sites can still drop down to cli-specific helpers
// when the abstraction would be lossy (e.g. ParseGlobalParams reads --random
// from the root via GlobalBool).
type V1 struct {
	Ctx *cli.Context
}

// NewV1 wraps a *cli.Context as Flags. Returning Flags (interface) lets
// callers swap adapters without touching parser signatures.
func NewV1(ctx *cli.Context) Flags { return V1{Ctx: ctx} }

// NewV1FromApp wraps the application-level (root) *cli.Context as Flags.
// Use it from app.Before / app.After callbacks and any other call site that
// already holds the root context and reads global flags (e.g. cmd/main.go).
// Semantically equivalent to NewV1(ctx).Global(); the dedicated constructor
// signals intent — "this context is the root" — without forcing readers to
// chase a Global() call to confirm.
func NewV1FromApp(ctx *cli.Context) Flags { return V1{Ctx: ctx} }

// String returns the value of the named string flag.
func (f V1) String(name string) string { return f.Ctx.String(name) }

// Bool returns the value of the named bool flag (defaults false).
func (f V1) Bool(name string) bool { return f.Ctx.Bool(name) }

// BoolT returns the value of the named "bool-true" flag (defaults true).
func (f V1) BoolT(name string) bool { return f.Ctx.BoolT(name) }

// Duration returns the value of the named duration flag.
func (f V1) Duration(name string) time.Duration { return f.Ctx.Duration(name) }

// Int returns the value of the named int flag.
func (f V1) Int(name string) int { return f.Ctx.Int(name) }

// Float64 returns the value of the named float64 flag.
func (f V1) Float64(name string) float64 { return f.Ctx.Float64(name) }

// StringSlice returns the value of the named string-slice flag.
func (f V1) StringSlice(name string) []string { return f.Ctx.StringSlice(name) }

// Args returns positional arguments as a plain []string.
func (f V1) Args() []string { return []string(f.Ctx.Args()) }

// Parent returns the parent subcommand's flags, or nil at the root.
func (f V1) Parent() Flags {
	p := f.Ctx.Parent()
	if p == nil {
		return nil
	}
	return V1{Ctx: p}
}

// Global walks up the parent chain and returns the root context's flags.
// Mirrors urfave/cli v1's GlobalX semantics enough for parser use.
func (f V1) Global() Flags {
	cur := f.Ctx
	for cur.Parent() != nil {
		cur = cur.Parent()
	}
	return V1{Ctx: cur}
}
