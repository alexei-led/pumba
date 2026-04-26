package cmd

import (
	"context"
	"flag"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func nilRuntime() chaos.Runtime {
	return func() container.Client { return nil }
}

func fakeRuntime(t *testing.T) (chaos.Runtime, *container.MockClient, *int) {
	t.Helper()
	want := container.NewMockClient(t)
	calls := 0
	rt := chaos.Runtime(func() container.Client {
		calls++
		return want
	})
	return rt, want, &calls
}

func defaultGlobalParams() *chaos.GlobalParams {
	return &chaos.GlobalParams{}
}

func assertConstructorContract(t *testing.T, cmd *cli.Command, wantName string) {
	t.Helper()
	require.NotNil(t, cmd, "constructor returned nil command")
	assert.Equal(t, wantName, cmd.Name)
	assert.NotNil(t, cmd.Action, "constructor must wire an Action closure")
}

// iptablesParentFlags mirrors the parent "iptables" command flags declared in
// cmd/main.go so subcommand parsers can hydrate via c.Parent().
func iptablesParentFlags() []cli.Flag {
	return []cli.Flag{
		cli.DurationFlag{Name: "duration, d"},
		cli.StringFlag{Name: "interface, i", Value: "eth0"},
		cli.StringFlag{Name: "protocol, p", Value: "any"},
		cli.StringSliceFlag{Name: "source, src, s"},
		cli.StringSliceFlag{Name: "destination, dest"},
		cli.StringFlag{Name: "src-port, sport"},
		cli.StringFlag{Name: "dst-port, dport"},
		cli.StringFlag{Name: "iptables-image", Value: "ghcr.io/alexei-led/pumba-alpine-nettools:latest"},
		cli.BoolTFlag{Name: "pull-image"},
	}
}

func iptablesParentContext(t *testing.T, args []string) *cli.Context {
	t.Helper()
	app := cli.NewApp()
	fs := flag.NewFlagSet("iptables", flag.ContinueOnError)
	for _, f := range iptablesParentFlags() {
		f.Apply(fs)
	}
	if args == nil {
		args = []string{"--duration", "1s", "--interface", "eth0"}
	}
	require.NoError(t, fs.Parse(args))
	return cli.NewContext(app, fs, nil)
}

func childContext(t *testing.T, parent *cli.Context, subFlags []cli.Flag, args []string) *cli.Context {
	t.Helper()
	fs := flag.NewFlagSet("sub", flag.ContinueOnError)
	for _, f := range subFlags {
		f.Apply(fs)
	}
	require.NoError(t, fs.Parse(args))
	return cli.NewContext(parent.App, fs, parent)
}

// ---- Loss ----------------------------------------------------------------

func TestNewLossCLICommand_Contract(t *testing.T) {
	rt, _, calls := fakeRuntime(t)
	cmd := NewLossCLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "loss")
	assert.Equal(t, 0, *calls, "Runtime must not be resolved at construction time")
}

func TestNewLossCLICommand_AcceptsNilRuntime(t *testing.T) {
	assert.NotNil(t, NewLossCLICommand(context.Background(), nilRuntime()))
}

func TestParseLossParams(t *testing.T) {
	cmd := NewLossCLICommand(context.Background(), nilRuntime())
	parent := iptablesParentContext(t, nil)
	c := childContext(t, parent, cmd.Flags,
		[]string{"--mode", "random", "--probability", "0.5"})
	got, err := parseLossParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.Equal(t, "random", got.Mode)
	assert.InDelta(t, 0.5, got.Probability, 0.001)
	require.NotNil(t, got.IPTables)
	assert.Equal(t, "eth0", got.IPTables.Iface)
}

func TestParseLossParams_BadIPTablesErrors(t *testing.T) {
	cmd := NewLossCLICommand(context.Background(), nilRuntime())
	parent := iptablesParentContext(t, []string{"--interface", "eth0"})
	c := childContext(t, parent, cmd.Flags, nil)
	_, err := parseLossParams(cliflags.NewV1(c), defaultGlobalParams())
	assert.ErrorContains(t, err, "duration")
}

func TestBuildLossCommand_Random(t *testing.T) {
	client := container.NewMockClient(t)
	parent := iptablesParentContext(t, nil)
	cmd := NewLossCLICommand(context.Background(), nilRuntime())
	c := childContext(t, parent, cmd.Flags, []string{"--mode", "random", "--probability", "0.5"})
	p, err := parseLossParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	built, err := buildLossCommand(client, defaultGlobalParams(), p)
	require.NoError(t, err)
	require.NotNil(t, built)
}

func TestBuildLossCommand_NTH(t *testing.T) {
	client := container.NewMockClient(t)
	parent := iptablesParentContext(t, nil)
	cmd := NewLossCLICommand(context.Background(), nilRuntime())
	c := childContext(t, parent, cmd.Flags,
		[]string{"--mode", "nth", "--every", "3", "--packet", "0"})
	p, err := parseLossParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	built, err := buildLossCommand(client, defaultGlobalParams(), p)
	require.NoError(t, err)
	require.NotNil(t, built)
}

func TestBuildLossCommand_BadMode(t *testing.T) {
	client := container.NewMockClient(t)
	parent := iptablesParentContext(t, nil)
	cmd := NewLossCLICommand(context.Background(), nilRuntime())
	c := childContext(t, parent, cmd.Flags, []string{"--mode", "bogus"})
	p, err := parseLossParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	_, err = buildLossCommand(client, defaultGlobalParams(), p)
	assert.Error(t, err)
}
