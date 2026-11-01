package cliflags

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/urfave/cli"
)

// Flag value types, named independently of the cli library. They are part of
// the pumba.cli/v1 schema contract.
const (
	TypeString      = "string"
	TypeBool        = "bool"
	TypeInt         = "int"
	TypeFloat       = "number"
	TypeDuration    = "duration"
	TypeStringSlice = "string[]"
)

// helpCommandName is the help command urfave/cli v1 adds to every app; the
// library does not export it.
const helpCommandName = "help"

// ErrUnsupportedFlagType reports a declared flag whose type DescribeV1 does
// not know, so it cannot be described.
var ErrUnsupportedFlagType = errors.New("unsupported flag type")

// FlagInfo describes a declared flag. The JSON form is the pumba.cli/v1 flag.
type FlagInfo struct {
	// Name is the primary flag name, without dashes.
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
	Type    string   `json:"type"`
	// Default is the typed default value; durations are rendered as strings ("0s").
	Default    any      `json:"default"`
	EnvVars    []string `json:"env_vars"`
	Repeatable bool     `json:"repeatable"`
	Usage      string   `json:"usage"`
	Hidden     bool     `json:"-"`
}

// CommandInfo describes a declared command and its subcommands. The JSON form
// is the pumba.cli/v1 command.
type CommandInfo struct {
	Name string `json:"name"`
	// Path is the full command path below the app, e.g. "netem delay".
	Path        string        `json:"path"`
	Usage       string        `json:"usage"`
	Description string        `json:"description"`
	ArgsUsage   string        `json:"args_usage"`
	Flags       []FlagInfo    `json:"flags"`
	Subcommands []CommandInfo `json:"subcommands"`
}

// ValueReader reads a flag's typed value from a parsed context.
type ValueReader func(c *cli.Context, name string) any

// DescribeV1 describes a urfave/cli v1 flag and returns the reader for its
// typed value. ok is false for flag types Pumba does not declare. It is the
// single flag type switch: CommandParams (run.start params) and the
// `pumba schema` walk (DescribeFlagsV1) both use it.
func DescribeV1(f cli.Flag) (FlagInfo, ValueReader, bool) {
	var info FlagInfo
	var read ValueReader
	switch fl := f.(type) {
	case cli.StringFlag:
		info, read = flagInfo(TypeString, fl.Value, fl.Usage, fl.EnvVar, fl.Hidden), func(c *cli.Context, n string) any { return c.String(n) }
	case cli.BoolFlag:
		info, read = flagInfo(TypeBool, false, fl.Usage, fl.EnvVar, fl.Hidden), func(c *cli.Context, n string) any { return c.Bool(n) }
	case cli.BoolTFlag:
		info, read = flagInfo(TypeBool, true, fl.Usage, fl.EnvVar, fl.Hidden), func(c *cli.Context, n string) any { return c.BoolT(n) }
	case cli.IntFlag:
		info, read = flagInfo(TypeInt, fl.Value, fl.Usage, fl.EnvVar, fl.Hidden), func(c *cli.Context, n string) any { return c.Int(n) }
	case cli.Float64Flag:
		info, read = flagInfo(TypeFloat, fl.Value, fl.Usage, fl.EnvVar, fl.Hidden), func(c *cli.Context, n string) any { return c.Float64(n) }
	case cli.DurationFlag:
		info, read = flagInfo(TypeDuration, fl.Value.String(), fl.Usage, fl.EnvVar, fl.Hidden), func(c *cli.Context, n string) any { return c.Duration(n).String() }
	case cli.StringSliceFlag:
		def := []string{}
		if fl.Value != nil {
			def = append(def, *fl.Value...)
		}
		info, read = flagInfo(TypeStringSlice, def, fl.Usage, fl.EnvVar, fl.Hidden), func(c *cli.Context, n string) any {
			if v := c.StringSlice(n); v != nil {
				return v
			}
			return []string{}
		}
		info.Repeatable = true
	default:
		return FlagInfo{}, nil, false
	}
	names := splitList(f.GetName())
	info.Name, info.Aliases = names[0], names[1:]
	return info, read, true
}

func flagInfo(typ string, def any, usage, envVar string, hidden bool) FlagInfo {
	env := []string{}
	if envVar != "" {
		env = splitList(envVar)
	}
	return FlagInfo{Type: typ, Default: def, EnvVars: env, Usage: usage, Hidden: hidden}
}

// splitList splits urfave's comma-separated flag name and env var lists.
func splitList(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func helpFlagName() string {
	return splitList(cli.HelpFlag.GetName())[0]
}

// DescribeFlagsV1 describes flags in declaration order, omitting hidden flags
// and the help flag. A flag of a type DescribeV1 does not know is an error, so
// it cannot silently drop out of the schema.
func DescribeFlagsV1(flags []cli.Flag) ([]FlagInfo, error) {
	out := []FlagInfo{}
	for _, f := range flags {
		info, _, ok := DescribeV1(f)
		if !ok {
			return nil, fmt.Errorf("flag %q (%T): %w", f.GetName(), f, ErrUnsupportedFlagType)
		}
		if info.Hidden || info.Name == helpFlagName() {
			continue
		}
		out = append(out, info)
	}
	return out, nil
}

// DescribeCommandsV1 describes cmds and their subcommands recursively in
// declaration order, omitting hidden commands and the help command. parent is
// the path of the command that owns cmds ("" for the app's commands).
func DescribeCommandsV1(cmds []cli.Command, parent string) ([]CommandInfo, error) {
	out := []CommandInfo{}
	for i := range cmds {
		c := &cmds[i]
		if c.Hidden || c.Name == helpCommandName {
			continue
		}
		path := strings.TrimSpace(parent + " " + c.Name)
		flags, err := DescribeFlagsV1(c.Flags)
		if err != nil {
			return nil, fmt.Errorf("command %q: %w", path, err)
		}
		subs, err := DescribeCommandsV1(c.Subcommands, path)
		if err != nil {
			return nil, err
		}
		out = append(out, CommandInfo{
			Name: c.Name, Path: path, Usage: c.Usage, Description: c.Description, ArgsUsage: c.ArgsUsage,
			Flags: flags, Subcommands: subs,
		})
	}
	return out, nil
}

// CommandParams returns the typed values of the flags declared by the running
// command and its parent commands, keyed by primary flag name. It never reads
// the application-level (global) flags, so global URLs, credentials and TLS
// paths cannot leak into it. Durations are rendered as strings ("30s").
// exclude drops flags by primary name, e.g. free text that may carry secrets.
func (f V1) CommandParams(exclude ...string) map[string]any {
	skip := map[string]bool{helpFlagName(): true}
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
