package cliflags

import (
	"slices"
	"strings"

	"github.com/urfave/cli"
)

// Flag value types, named independently of the cli library.
const (
	TypeString      = "string"
	TypeBool        = "bool"
	TypeInt         = "int"
	TypeFloat       = "float"
	TypeDuration    = "duration"
	TypeStringSlice = "string_slice"
)

// FlagInfo describes a declared flag.
type FlagInfo struct {
	// Name is the primary flag name, without dashes.
	Name    string
	Aliases []string
	Type    string
}

// ValueReader reads a flag's typed value from a parsed context.
type ValueReader func(c *cli.Context, name string) any

// DescribeV1 describes a urfave/cli v1 flag and returns the reader for its
// typed value. ok is false for flag types Pumba does not declare. It is the
// single flag type switch: CommandParams (run.start params) uses it, and
// `pumba schema` reuses it for flag introspection.
func DescribeV1(f cli.Flag) (FlagInfo, ValueReader, bool) {
	var typ string
	var read ValueReader
	switch f.(type) {
	case cli.StringFlag:
		typ, read = TypeString, func(c *cli.Context, n string) any { return c.String(n) }
	case cli.BoolFlag:
		typ, read = TypeBool, func(c *cli.Context, n string) any { return c.Bool(n) }
	case cli.BoolTFlag:
		typ, read = TypeBool, func(c *cli.Context, n string) any { return c.BoolT(n) }
	case cli.IntFlag:
		typ, read = TypeInt, func(c *cli.Context, n string) any { return c.Int(n) }
	case cli.Float64Flag:
		typ, read = TypeFloat, func(c *cli.Context, n string) any { return c.Float64(n) }
	case cli.DurationFlag:
		typ, read = TypeDuration, func(c *cli.Context, n string) any { return c.Duration(n).String() }
	case cli.StringSliceFlag:
		typ, read = TypeStringSlice, func(c *cli.Context, n string) any {
			if v := c.StringSlice(n); v != nil {
				return v
			}
			return []string{}
		}
	default:
		return FlagInfo{}, nil, false
	}
	names := strings.Split(f.GetName(), ",")
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}
	return FlagInfo{Name: names[0], Aliases: names[1:], Type: typ}, read, true
}

// CommandParams returns the typed values of the flags declared by the running
// command and its parent commands, keyed by primary flag name. It never reads
// the application-level (global) flags, so global URLs, credentials and TLS
// paths cannot leak into it. Durations are rendered as strings ("30s").
// exclude drops flags by primary name, e.g. free text that may carry secrets.
func (f V1) CommandParams(exclude ...string) map[string]any {
	skip := map[string]bool{strings.Split(cli.HelpFlag.GetName(), ",")[0]: true}
	for _, name := range exclude {
		skip[name] = true
	}
	// Walk up to, but not including, the root context. Parents are read first
	// so a subcommand flag wins over a same-named parent flag.
	var chain []*cli.Context
	for cur := f.Ctx; cur != nil && cur.Parent() != nil; cur = cur.Parent() {
		chain = append(chain, cur)
	}
	params := map[string]any{}
	for i, ctx := range slices.Backward(chain) {
		flags := ctx.App.Flags // a parent command runs as a sub-app
		if i == 0 {
			flags = ctx.Command.Flags
		}
		for _, fl := range flags {
			info, read, ok := DescribeV1(fl)
			if !ok || skip[info.Name] {
				continue
			}
			params[info.Name] = read(ctx, info.Name)
		}
	}
	return params
}
