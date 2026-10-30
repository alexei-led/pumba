package cliflags_test

import (
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func TestDescribeV1(t *testing.T) {
	tests := []struct {
		flag cli.Flag
		want cliflags.FlagInfo
		ok   bool
	}{
		{cli.StringFlag{Name: "tc-image"}, cliflags.FlagInfo{Name: "tc-image", Aliases: []string{}, Type: cliflags.TypeString}, true},
		{cli.BoolFlag{Name: "restart, r"}, cliflags.FlagInfo{Name: "restart", Aliases: []string{"r"}, Type: cliflags.TypeBool}, true},
		{cli.BoolTFlag{Name: "pull-image"}, cliflags.FlagInfo{Name: "pull-image", Aliases: []string{}, Type: cliflags.TypeBool}, true},
		{cli.IntFlag{Name: "limit, l"}, cliflags.FlagInfo{Name: "limit", Aliases: []string{"l"}, Type: cliflags.TypeInt}, true},
		{cli.Float64Flag{Name: "correlation, c"}, cliflags.FlagInfo{Name: "correlation", Aliases: []string{"c"}, Type: cliflags.TypeFloat}, true},
		{cli.DurationFlag{Name: "duration, d"}, cliflags.FlagInfo{Name: "duration", Aliases: []string{"d"}, Type: cliflags.TypeDuration}, true},
		{cli.StringSliceFlag{Name: "source, src, s"}, cliflags.FlagInfo{Name: "source", Aliases: []string{"src", "s"}, Type: cliflags.TypeStringSlice}, true},
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
