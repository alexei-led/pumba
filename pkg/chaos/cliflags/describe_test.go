package cliflags_test

import (
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func TestDescribeV1(t *testing.T) {
	none := []string{}
	tests := []struct {
		flag cli.Flag
		want cliflags.FlagInfo
		ok   bool
	}{
		{
			cli.StringFlag{Name: "tc-image", Value: "img:latest", Usage: "nettools image"},
			cliflags.FlagInfo{Name: "tc-image", Aliases: none, Type: "string", Default: "img:latest", EnvVars: none, Usage: "nettools image"},
			true,
		},
		{
			cli.StringFlag{Name: "host, H", EnvVar: "DOCKER_HOST, PUMBA_HOST"},
			cliflags.FlagInfo{Name: "host", Aliases: []string{"H"}, Type: "string", Default: "", EnvVars: []string{"DOCKER_HOST", "PUMBA_HOST"}},
			true,
		},
		{cli.BoolFlag{Name: "restart, r"}, cliflags.FlagInfo{Name: "restart", Aliases: []string{"r"}, Type: "bool", Default: false, EnvVars: none}, true},
		{cli.BoolTFlag{Name: "pull-image"}, cliflags.FlagInfo{Name: "pull-image", Aliases: none, Type: "bool", Default: true, EnvVars: none}, true},
		{cli.IntFlag{Name: "limit, l", Value: 3}, cliflags.FlagInfo{Name: "limit", Aliases: []string{"l"}, Type: "int", Default: 3, EnvVars: none}, true},
		{cli.Float64Flag{Name: "correlation, c", Value: 0.5}, cliflags.FlagInfo{Name: "correlation", Aliases: []string{"c"}, Type: "number", Default: 0.5, EnvVars: none}, true},
		{cli.DurationFlag{Name: "duration, d"}, cliflags.FlagInfo{Name: "duration", Aliases: []string{"d"}, Type: "duration", Default: "0s", EnvVars: none}, true},
		{
			cli.StringSliceFlag{Name: "source, src, s"},
			cliflags.FlagInfo{Name: "source", Aliases: []string{"src", "s"}, Type: "string[]", Default: none, EnvVars: none, Repeatable: true},
			true,
		},
		{
			cli.StringSliceFlag{Name: "label", Value: &cli.StringSlice{"a=b"}},
			cliflags.FlagInfo{Name: "label", Aliases: none, Type: "string[]", Default: []string{"a=b"}, EnvVars: none, Repeatable: true},
			true,
		},
		{cli.BoolFlag{Name: "secret", Hidden: true}, cliflags.FlagInfo{Name: "secret", Aliases: none, Type: "bool", Default: false, EnvVars: none, Hidden: true}, true},
		{cli.UintFlag{Name: "unused"}, cliflags.FlagInfo{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.flag.GetName(), func(t *testing.T) {
			got, _, ok := cliflags.DescribeV1(tt.flag)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDescribeV1DefaultsMatchReaders(t *testing.T) {
	flags := []cli.Flag{
		cli.StringFlag{Name: "text", Value: "default"},
		cli.BoolFlag{Name: "disabled"},
		cli.BoolTFlag{Name: "enabled"},
		cli.IntFlag{Name: "integer", Value: 12},
		cli.Float64Flag{Name: "number", Value: 0.5},
		cli.DurationFlag{Name: "duration", Value: 30 * time.Second},
		cli.StringSliceFlag{Name: "empty-list"},
		cli.StringSliceFlag{Name: "list", Value: &cli.StringSlice{"a", "b"}},
	}
	ctx := newCtx(t, flags, nil)
	for _, flag := range flags {
		t.Run(flag.GetName(), func(t *testing.T) {
			info, read, ok := cliflags.DescribeV1(flag)
			require.True(t, ok)
			assert.Equal(t, info.Default, read(ctx, info.Name))
		})
	}
}

func TestDescribeFlagsV1OmitsHiddenAndHelp(t *testing.T) {
	got, err := cliflags.DescribeFlagsV1([]cli.Flag{
		cli.IntFlag{Name: "b"},
		cli.BoolFlag{Name: "hidden", Hidden: true},
		cli.HelpFlag,
		cli.StringFlag{Name: "a"},
	})
	require.NoError(t, err)
	names := make([]string, 0, len(got))
	for _, f := range got {
		names = append(names, f.Name)
	}
	assert.Equal(t, []string{"b", "a"}, names, "declaration order, hidden and help omitted")
}

func TestDescribeFlagsV1EmptyIsNotNull(t *testing.T) {
	got, err := cliflags.DescribeFlagsV1(nil)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestDescribeV1RejectsUnknownFlagTypes(t *testing.T) {
	tests := []struct {
		name    string
		cmds    []cli.Command
		wantErr string
	}{
		{
			name:    "top-level command",
			cmds:    []cli.Command{{Name: "kill", Flags: []cli.Flag{cli.UintFlag{Name: "signal"}}}},
			wantErr: `command "kill": flag "signal" (cli.UintFlag): unsupported flag type`,
		},
		{
			name: "subcommand",
			cmds: []cli.Command{{Name: "netem", Subcommands: []cli.Command{
				{Name: "delay", Flags: []cli.Flag{cli.Int64Flag{Name: "time, t"}}},
			}}},
			wantErr: `command "netem delay": flag "time, t" (cli.Int64Flag): unsupported flag type`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cliflags.DescribeCommandsV1(tt.cmds, "")
			require.ErrorIs(t, err, cliflags.ErrUnsupportedFlagType)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestDescribeCommandsV1(t *testing.T) {
	cmds := []cli.Command{
		{Name: "kill", Usage: "kill", ArgsUsage: "containers", Flags: []cli.Flag{cli.StringFlag{Name: "signal, s", Value: "SIGKILL"}}},
		{Name: "secret", Hidden: true},
		{Name: "help", Usage: "Shows a list of commands or help for one command"},
		{
			Name: "netem", Description: "netem desc",
			Flags:       []cli.Flag{cli.BoolTFlag{Name: "pull-image"}},
			Subcommands: []cli.Command{{Name: "delay"}, {Name: "loss"}},
		},
	}
	got, err := cliflags.DescribeCommandsV1(cmds, "")
	require.NoError(t, err)
	none := []string{}
	want := []cliflags.CommandInfo{
		{
			Name: "kill", Path: "kill", Usage: "kill", ArgsUsage: "containers",
			Flags:       []cliflags.FlagInfo{{Name: "signal", Aliases: []string{"s"}, Type: "string", Default: "SIGKILL", EnvVars: none}},
			Subcommands: []cliflags.CommandInfo{},
		},
		{
			Name: "netem", Path: "netem", Description: "netem desc",
			Flags: []cliflags.FlagInfo{{Name: "pull-image", Aliases: none, Type: "bool", Default: true, EnvVars: none}},
			Subcommands: []cliflags.CommandInfo{
				{Name: "delay", Path: "netem delay", Flags: []cliflags.FlagInfo{}, Subcommands: []cliflags.CommandInfo{}},
				{Name: "loss", Path: "netem loss", Flags: []cliflags.FlagInfo{}, Subcommands: []cliflags.CommandInfo{}},
			},
		},
	}
	assert.Equal(t, want, got)
}

func TestCommandParamsTopLevelCommandSkipsGlobals(t *testing.T) {
	var got map[string]any
	app := cli.NewApp()
	app.Flags = []cli.Flag{cli.StringFlag{Name: "tlskey"}, cli.StringFlag{Name: "slackhook"}}
	app.Commands = []cli.Command{{
		Name: "stop",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "duration, d", Value: "10s"},
			cli.BoolFlag{Name: "restart, r"},
			cli.IntFlag{Name: "limit, l"},
		},
		Action: func(c *cli.Context) error {
			got = cliflags.V1{Ctx: c}.CommandParams()
			return nil
		},
	}}
	require.NoError(t, app.Run([]string{"pumba", "--tlskey", "/k.pem", "--slackhook", "https://hooks/x", "stop", "-r", "c1"}))
	assert.Equal(t, map[string]any{"duration": "10s", "restart": true, "limit": 0}, got)
}

func TestCommandParamsSubcommandOverridesParent(t *testing.T) {
	var got map[string]any
	app := cli.NewApp()
	app.Commands = []cli.Command{{
		Name:  "parent",
		Flags: []cli.Flag{cli.IntFlag{Name: "limit", Value: 1}, cli.StringFlag{Name: "secret"}},
		Subcommands: []cli.Command{{
			Name:  "child",
			Flags: []cli.Flag{cli.IntFlag{Name: "limit", Value: 7}},
			Action: func(c *cli.Context) error {
				got = cliflags.V1{Ctx: c}.CommandParams("secret")
				return nil
			},
		}},
	}}
	require.NoError(t, app.Run([]string{"pumba", "parent", "--secret", "s3cr3t", "child"}))
	assert.Equal(t, map[string]any{"limit": 7}, got)
}

func TestCommandParamsAtRootIsEmpty(t *testing.T) {
	app := newCtx(t, []cli.Flag{cli.StringFlag{Name: "host"}}, []string{"--host", "tcp://x"})
	assert.Empty(t, cliflags.V1{Ctx: app}.CommandParams())
}
