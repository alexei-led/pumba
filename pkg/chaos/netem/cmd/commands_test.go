package cmd

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

// newTestCLIContext builds a *cli.Context whose flag set carries the given
// flags and parsed values. Pass `args` like `[]string{"--percent", "50"}`.
func newTestCLIContext(t *testing.T, flags []cli.Flag, args []string) *cli.Context {
	t.Helper()
	app := cli.NewApp()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range flags {
		f.Apply(fs)
	}
	require.NoError(t, fs.Parse(args))
	return cli.NewContext(app, fs, nil)
}

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

// netemContext returns a parent CLI context populated with valid netem-level
// flags (--duration, --interface, --tc-image, …) so subcommand parsers can
// hydrate via c.Parent(). Override individual values by passing args.
func netemContext(t *testing.T, args []string) *cli.Context {
	t.Helper()
	app := cli.NewApp()
	fs := flag.NewFlagSet("netem", flag.ContinueOnError)
	for _, f := range netemFlags() {
		f.Apply(fs)
	}
	if args == nil {
		args = []string{"--duration", "1s", "--interface", "eth0"}
	}
	require.NoError(t, fs.Parse(args))
	return cli.NewContext(app, fs, nil)
}

func netemFlags() []cli.Flag {
	return []cli.Flag{
		cli.DurationFlag{Name: "duration, d"},
		cli.StringFlag{Name: "interface, i", Value: "eth0"},
		cli.StringSliceFlag{Name: "target, t"},
		cli.StringFlag{Name: "egress-port, egressPort"},
		cli.StringFlag{Name: "ingress-port, ingressPort"},
		cli.StringFlag{Name: "tc-image", Value: "ghcr.io/alexei-led/pumba-alpine-nettools:latest"},
		cli.BoolTFlag{Name: "pull-image"},
		cli.IntFlag{Name: "limit"},
	}
}

// childContext mounts subFlags as a child of a populated netem parent
// context, so c.Parent() resolves correctly inside parse functions.
func childContext(t *testing.T, parent *cli.Context, subFlags []cli.Flag, args []string) *cli.Context {
	t.Helper()
	fs := flag.NewFlagSet("sub", flag.ContinueOnError)
	for _, f := range subFlags {
		f.Apply(fs)
	}
	require.NoError(t, fs.Parse(args))
	return cli.NewContext(parent.App, fs, parent)
}

// ---- Delay ---------------------------------------------------------------

func TestNewDelayCLICommand_Contract(t *testing.T) {
	rt, _, calls := fakeRuntime(t)
	cmd := NewDelayCLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "delay")
	assert.Equal(t, 0, *calls, "Runtime must not be resolved at construction time")
}

func TestParseDelayParams(t *testing.T) {
	cmd := NewDelayCLICommand(context.Background(), nilRuntime())
	parent := netemContext(t, nil)
	c := childContext(t, parent, cmd.Flags,
		[]string{"--time", "200", "--jitter", "20", "--correlation", "30", "--distribution", "normal"})
	got, err := parseDelayParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.Equal(t, 200, got.Time)
	assert.Equal(t, 20, got.Jitter)
	assert.InDelta(t, 30.0, got.Correlation, 0.001)
	assert.Equal(t, "normal", got.Distribution)
	require.NotNil(t, got.Base)
	assert.Equal(t, "eth0", got.Base.Interface)
}

func TestParseDelayParams_BadNetemErrors(t *testing.T) {
	cmd := NewDelayCLICommand(context.Background(), nilRuntime())
	// duration unset on parent → parseNetemParams must surface the error
	parent := netemContext(t, []string{"--interface", "eth0"})
	c := childContext(t, parent, cmd.Flags, nil)
	_, err := parseDelayParams(cliflags.NewV1(c), defaultGlobalParams())
	assert.ErrorContains(t, err, "duration")
}

func TestBuildDelayCommand(t *testing.T) {
	client := container.NewMockClient(t)
	parent := netemContext(t, nil)
	cmd := NewDelayCLICommand(context.Background(), nilRuntime())
	c := childContext(t, parent, cmd.Flags, nil)
	p, err := parseDelayParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	built, err := buildDelayCommand(client, defaultGlobalParams(), p)
	require.NoError(t, err)
	require.NotNil(t, built)
}

// ---- Loss ----------------------------------------------------------------

func TestNewLossCLICommand_Contract(t *testing.T) {
	rt, _, calls := fakeRuntime(t)
	cmd := NewLossCLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "loss")
	assert.Equal(t, 0, *calls)
}

func TestParseLossParams(t *testing.T) {
	cmd := NewLossCLICommand(context.Background(), nilRuntime())
	parent := netemContext(t, nil)
	c := childContext(t, parent, cmd.Flags, []string{"--percent", "10", "--correlation", "5"})
	got, err := parseLossParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.InDelta(t, 10.0, got.Percent, 0.001)
	assert.InDelta(t, 5.0, got.Correlation, 0.001)
}

func TestBuildLossCommand(t *testing.T) {
	client := container.NewMockClient(t)
	parent := netemContext(t, nil)
	cmd := NewLossCLICommand(context.Background(), nilRuntime())
	c := childContext(t, parent, cmd.Flags, []string{"--percent", "10"})
	p, err := parseLossParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	built, err := buildLossCommand(client, defaultGlobalParams(), p)
	require.NoError(t, err)
	require.NotNil(t, built)
}

// ---- Loss-State ----------------------------------------------------------

func TestNewLossStateCLICommand_Contract(t *testing.T) {
	rt, _, _ := fakeRuntime(t)
	cmd := NewLossStateCLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "loss-state")
}

func TestParseLossStateParams(t *testing.T) {
	cmd := NewLossStateCLICommand(context.Background(), nilRuntime())
	parent := netemContext(t, nil)
	c := childContext(t, parent, cmd.Flags,
		[]string{"--p13", "10", "--p31", "90", "--p32", "5", "--p23", "85", "--p14", "1"})
	got, err := parseLossStateParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.InDelta(t, 10.0, got.P13, 0.001)
	assert.InDelta(t, 90.0, got.P31, 0.001)
	assert.InDelta(t, 5.0, got.P32, 0.001)
	assert.InDelta(t, 85.0, got.P23, 0.001)
	assert.InDelta(t, 1.0, got.P14, 0.001)
}

func TestBuildLossStateCommand(t *testing.T) {
	client := container.NewMockClient(t)
	parent := netemContext(t, nil)
	cmd := NewLossStateCLICommand(context.Background(), nilRuntime())
	c := childContext(t, parent, cmd.Flags, nil)
	p, err := parseLossStateParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	built, err := buildLossStateCommand(client, defaultGlobalParams(), p)
	require.NoError(t, err)
	require.NotNil(t, built)
}

// ---- Loss-GE -------------------------------------------------------------

func TestNewLossGECLICommand_Contract(t *testing.T) {
	rt, _, _ := fakeRuntime(t)
	cmd := NewLossGECLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "loss-gemodel")
}

func TestParseLossGEParams(t *testing.T) {
	cmd := NewLossGECLICommand(context.Background(), nilRuntime())
	parent := netemContext(t, nil)
	c := childContext(t, parent, cmd.Flags,
		[]string{"--pg", "10", "--pb", "90", "--one-h", "80", "--one-k", "5"})
	got, err := parseLossGEParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.InDelta(t, 10.0, got.PG, 0.001)
	assert.InDelta(t, 90.0, got.PB, 0.001)
	assert.InDelta(t, 80.0, got.OneH, 0.001)
	assert.InDelta(t, 5.0, got.OneK, 0.001)
}

func TestBuildLossGECommand(t *testing.T) {
	client := container.NewMockClient(t)
	parent := netemContext(t, nil)
	cmd := NewLossGECLICommand(context.Background(), nilRuntime())
	c := childContext(t, parent, cmd.Flags, nil)
	p, err := parseLossGEParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	built, err := buildLossGECommand(client, defaultGlobalParams(), p)
	require.NoError(t, err)
	require.NotNil(t, built)
}

// ---- Rate ----------------------------------------------------------------

func TestNewRateCLICommand_Contract(t *testing.T) {
	rt, _, _ := fakeRuntime(t)
	cmd := NewRateCLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "rate")
}

func TestParseRateParams(t *testing.T) {
	cmd := NewRateCLICommand(context.Background(), nilRuntime())
	parent := netemContext(t, nil)
	c := childContext(t, parent, cmd.Flags,
		[]string{"--rate", "1mbit", "--packetoverhead", "1", "--cellsize", "2", "--celloverhead", "3"})
	got, err := parseRateParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.Equal(t, "1mbit", got.Rate)
	assert.Equal(t, 1, got.PacketOverhead)
	assert.Equal(t, 2, got.CellSize)
	assert.Equal(t, 3, got.CellOverhead)
}

func TestBuildRateCommand(t *testing.T) {
	client := container.NewMockClient(t)
	parent := netemContext(t, nil)
	cmd := NewRateCLICommand(context.Background(), nilRuntime())
	c := childContext(t, parent, cmd.Flags, []string{"--rate", "1mbit"})
	p, err := parseRateParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	built, err := buildRateCommand(client, defaultGlobalParams(), p)
	require.NoError(t, err)
	require.NotNil(t, built)
}

// ---- Duplicate -----------------------------------------------------------

func TestNewDuplicateCLICommand_Contract(t *testing.T) {
	rt, _, _ := fakeRuntime(t)
	cmd := NewDuplicateCLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "duplicate")
}

func TestParseDuplicateParams(t *testing.T) {
	cmd := NewDuplicateCLICommand(context.Background(), nilRuntime())
	parent := netemContext(t, nil)
	c := childContext(t, parent, cmd.Flags, []string{"--percent", "15", "--correlation", "5"})
	got, err := parseDuplicateParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.InDelta(t, 15.0, got.Percent, 0.001)
	assert.InDelta(t, 5.0, got.Correlation, 0.001)
}

func TestBuildDuplicateCommand(t *testing.T) {
	client := container.NewMockClient(t)
	parent := netemContext(t, nil)
	cmd := NewDuplicateCLICommand(context.Background(), nilRuntime())
	c := childContext(t, parent, cmd.Flags, []string{"--percent", "15"})
	p, err := parseDuplicateParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	built, err := buildDuplicateCommand(client, defaultGlobalParams(), p)
	require.NoError(t, err)
	require.NotNil(t, built)
}

// ---- Corrupt -------------------------------------------------------------

func TestNewCorruptCLICommand_Contract(t *testing.T) {
	rt, _, _ := fakeRuntime(t)
	cmd := NewCorruptCLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "corrupt")
}

func TestParseCorruptParams(t *testing.T) {
	cmd := NewCorruptCLICommand(context.Background(), nilRuntime())
	parent := netemContext(t, nil)
	c := childContext(t, parent, cmd.Flags, []string{"--percent", "20", "--correlation", "5"})
	got, err := parseCorruptParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.InDelta(t, 20.0, got.Percent, 0.001)
	assert.InDelta(t, 5.0, got.Correlation, 0.001)
}

func TestBuildCorruptCommand(t *testing.T) {
	client := container.NewMockClient(t)
	parent := netemContext(t, nil)
	cmd := NewCorruptCLICommand(context.Background(), nilRuntime())
	c := childContext(t, parent, cmd.Flags, []string{"--percent", "20"})
	p, err := parseCorruptParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	built, err := buildCorruptCommand(client, defaultGlobalParams(), p)
	require.NoError(t, err)
	require.NotNil(t, built)
}

// ---- Cross-cutting -------------------------------------------------------

func TestRuntimeAcceptsNil(t *testing.T) {
	rt := nilRuntime()
	assert.NotNil(t, NewDelayCLICommand(context.Background(), rt))
	assert.NotNil(t, NewLossCLICommand(context.Background(), rt))
	assert.NotNil(t, NewLossStateCLICommand(context.Background(), rt))
	assert.NotNil(t, NewLossGECLICommand(context.Background(), rt))
	assert.NotNil(t, NewRateCLICommand(context.Background(), rt))
	assert.NotNil(t, NewDuplicateCLICommand(context.Background(), rt))
	assert.NotNil(t, NewCorruptCLICommand(context.Background(), rt))
}

// TestParseRequestBaseRejectsBadInterval double-checks interval enforcement on
// the shared netem parser still trips the duration<interval invariant.
func TestParseRequestBaseRejectsBadInterval(t *testing.T) {
	c := newTestCLIContext(t, netemFlags(), []string{"--duration", "5s"})
	_, _, err := netem.ParseRequestBase(cliflags.NewV1(c), &chaos.GlobalParams{Interval: 5 * time.Second})
	assert.ErrorContains(t, err, "duration must be shorter than interval")
}
