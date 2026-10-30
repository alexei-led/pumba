package cmd

import (
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

type fakeChaos struct {
	runs int
	err  error
}

func (f *fakeChaos) Run(ctx context.Context, random bool) error {
	f.runs++
	return f.err
}

type testParams struct {
	Limit int
}

func newTestCLIContext(args []string) *cli.Context {
	app := cli.NewApp()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = fs.Parse(args)
	return cli.NewContext(app, fs, nil)
}

// runtimeReturning wraps a fixed client in a chaos.Runtime closure and exposes
// the call counter so tests can assert when (and how often) NewAction resolved
// the runtime.
func runtimeReturning(client container.Client) (chaos.Runtime, *int) {
	calls := 0
	return func() container.Client {
		calls++
		return client
	}, &calls
}

func actionFunc(t *testing.T, cmd *cli.Command) func(*cli.Context) error {
	t.Helper()
	fn, ok := cmd.Action.(func(*cli.Context) error)
	require.True(t, ok, "cmd.Action should be func(*cli.Context) error")
	return fn
}

func TestNewAction_PopulatesCLICommand(t *testing.T) {
	rt, _ := runtimeReturning(nil)
	spec := Spec[testParams]{
		Name:        "test",
		Usage:       "test usage",
		ArgsUsage:   "<containers>",
		Description: "test description",
		Flags:       []cli.Flag{cli.IntFlag{Name: "limit"}},
		Parse:       func(c cliflags.Flags, gp *chaos.GlobalParams) (testParams, error) { return testParams{}, nil },
		Build: func(client container.Client, gp *chaos.GlobalParams, p testParams) (chaos.Command, error) {
			return &fakeChaos{}, nil
		},
	}
	cmd := NewAction(context.Background(), rt, chaos.NopSink{}, spec)
	require.NotNil(t, cmd)
	assert.Equal(t, "test", cmd.Name)
	assert.Equal(t, "test usage", cmd.Usage)
	assert.Equal(t, "<containers>", cmd.ArgsUsage)
	assert.Equal(t, "test description", cmd.Description)
	assert.Len(t, cmd.Flags, 1)
	assert.NotNil(t, cmd.Action)
}

func TestNewAction_RequireArgs_NoneGiven(t *testing.T) {
	rt, calls := runtimeReturning(nil)
	parseCalled := false
	spec := Spec[testParams]{
		Name:        "test",
		RequireArgs: true,
		Parse: func(c cliflags.Flags, gp *chaos.GlobalParams) (testParams, error) {
			parseCalled = true
			return testParams{}, nil
		},
		Build: func(client container.Client, gp *chaos.GlobalParams, p testParams) (chaos.Command, error) {
			return &fakeChaos{}, nil
		},
	}
	cmd := NewAction(context.Background(), rt, chaos.NopSink{}, spec)
	err := actionFunc(t, cmd)(newTestCLIContext(nil))
	assert.ErrorIs(t, err, ErrContainerArgRequired)
	assert.False(t, parseCalled, "Parse must not run when RequireArgs check fails")
	assert.Equal(t, 0, *calls, "Runtime must not be resolved when RequireArgs check fails")
}

func TestNewAction_RequireArgs_Satisfied(t *testing.T) {
	rt, _ := runtimeReturning(container.NewMockClient(t))
	spec := Spec[testParams]{
		Name:        "test",
		RequireArgs: true,
		Parse:       func(c cliflags.Flags, gp *chaos.GlobalParams) (testParams, error) { return testParams{}, nil },
		Build: func(client container.Client, gp *chaos.GlobalParams, p testParams) (chaos.Command, error) {
			return &fakeChaos{}, nil
		},
	}
	cmd := NewAction(context.Background(), rt, chaos.NopSink{}, spec)
	err := actionFunc(t, cmd)(newTestCLIContext([]string{"foo"}))
	require.NoError(t, err)
}

func TestNewAction_ParseErrorPropagated(t *testing.T) {
	rt, calls := runtimeReturning(nil)
	parseErr := errors.New("bad params")
	spec := Spec[testParams]{
		Name: "test",
		Parse: func(c cliflags.Flags, gp *chaos.GlobalParams) (testParams, error) {
			return testParams{}, parseErr
		},
		Build: func(client container.Client, gp *chaos.GlobalParams, p testParams) (chaos.Command, error) {
			t.Fatal("Build must not run when Parse fails")
			return nil, nil
		},
	}
	cmd := NewAction(context.Background(), rt, chaos.NopSink{}, spec)
	err := actionFunc(t, cmd)(newTestCLIContext([]string{"foo"}))
	assert.ErrorIs(t, err, parseErr)
	assert.Equal(t, 0, *calls, "Runtime must not be resolved when Parse fails")
}

func TestNewAction_BuildErrorPropagated(t *testing.T) {
	want := container.NewMockClient(t)
	rt, calls := runtimeReturning(want)
	buildErr := errors.New("build failed")
	spec := Spec[testParams]{
		Name: "test",
		Parse: func(c cliflags.Flags, gp *chaos.GlobalParams) (testParams, error) {
			return testParams{Limit: 5}, nil
		},
		Build: func(client container.Client, gp *chaos.GlobalParams, p testParams) (chaos.Command, error) {
			return nil, buildErr
		},
	}
	cmd := NewAction(context.Background(), rt, chaos.NopSink{}, spec)
	err := actionFunc(t, cmd)(newTestCLIContext([]string{"foo"}))
	assert.ErrorIs(t, err, buildErr)
	assert.Equal(t, 1, *calls, "Runtime must be resolved exactly once before Build")
}

func TestNewAction_HappyPath_ResolvesRuntimeAndRunsCommand(t *testing.T) {
	want := container.NewMockClient(t)
	rt, calls := runtimeReturning(want)
	chaosCmd := &fakeChaos{}

	var gotClient container.Client
	var gotParams testParams

	spec := Spec[testParams]{
		Name: "test",
		Parse: func(c cliflags.Flags, gp *chaos.GlobalParams) (testParams, error) {
			return testParams{Limit: 7}, nil
		},
		Build: func(client container.Client, gp *chaos.GlobalParams, p testParams) (chaos.Command, error) {
			gotClient = client
			gotParams = p
			return chaosCmd, nil
		},
	}
	cmd := NewAction(context.Background(), rt, chaos.NopSink{}, spec)
	err := actionFunc(t, cmd)(newTestCLIContext([]string{"foo"}))
	require.NoError(t, err)
	assert.Same(t, want, gotClient, "Build must receive the client returned by runtime()")
	assert.Equal(t, testParams{Limit: 7}, gotParams)
	assert.Equal(t, 1, *calls, "Runtime must be resolved exactly once")
	assert.Equal(t, 1, chaosCmd.runs, "Built command must be run via chaos.RunChaosCommand")
}

func TestNewAction_RunErrorWrapped(t *testing.T) {
	want := container.NewMockClient(t)
	rt, _ := runtimeReturning(want)
	runErr := errors.New("boom")

	spec := Spec[testParams]{
		Name: "ploop",
		Parse: func(c cliflags.Flags, gp *chaos.GlobalParams) (testParams, error) {
			return testParams{}, nil
		},
		Build: func(client container.Client, gp *chaos.GlobalParams, p testParams) (chaos.Command, error) {
			return &fakeChaos{err: runErr}, nil
		},
	}
	cmd := NewAction(context.Background(), rt, chaos.NopSink{}, spec)
	err := actionFunc(t, cmd)(newTestCLIContext([]string{"foo"}))
	require.Error(t, err)
	assert.ErrorIs(t, err, runErr)
	assert.Contains(t, err.Error(), "ploop", "wrapped error should mention command name")
}

// recordSink keeps every emitted event in order.
type recordSink struct{ events []chaos.Event }

func (s *recordSink) Emit(ev *chaos.Event) { s.events = append(s.events, *ev) }

// runApp runs a pumba-shaped app: global flags, a parent command with flags,
// and the NewAction subcommand under it.
func runApp(t *testing.T, sub *cli.Command, args ...string) error {
	t.Helper()
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "run-id"},
		cli.StringFlag{Name: "runtime", Value: "docker"},
		cli.StringFlag{Name: "tlscacert"},
		cli.DurationFlag{Name: "interval, i"},
		cli.IntFlag{Name: "max-targets"},
		cli.StringSliceFlag{Name: "label"},
		cli.BoolFlag{Name: "dry-run"},
	}
	app.Commands = []cli.Command{{
		Name: "netem",
		Flags: []cli.Flag{
			cli.DurationFlag{Name: "duration, d"},
			cli.StringFlag{Name: "interface, i", Value: "eth0"},
			cli.StringSliceFlag{Name: "target, t"},
			cli.BoolTFlag{Name: "pull-image"},
		},
		Subcommands: []cli.Command{*sub},
	}}
	app.ExitErrHandler = func(*cli.Context, error) {}
	return app.Run(append([]string{"pumba"}, args...))
}

func TestNewActionEmitsRunStart(t *testing.T) {
	sink := &recordSink{}
	spec := Spec[testParams]{
		Name: "delay",
		Flags: []cli.Flag{
			cli.IntFlag{Name: "time, t", Value: 100},
			cli.Float64Flag{Name: "correlation, c"},
			cli.StringSliceFlag{Name: "args, a"},
		},
		ExcludeParams: []string{"args"},
		Parse:         func(cliflags.Flags, *chaos.GlobalParams) (testParams, error) { return testParams{}, nil },
		Build: func(container.Client, *chaos.GlobalParams, testParams) (chaos.Command, error) {
			return &fakeChaos{}, nil
		},
	}
	rt, _ := runtimeReturning(nil)
	err := runApp(t, NewAction(context.Background(), rt, sink, spec),
		"--run-id", "r1", "--tlscacert", "/secret/ca.pem", "--max-targets", "2", "--dry-run",
		"netem", "--duration", "30s", "delay", "--correlation", "2.5", "--args", "token", "web-1")
	require.NoError(t, err)

	require.Len(t, sink.events, 1)
	ev := sink.events[0]
	assert.Equal(t, chaos.EventRunStart, ev.Type)
	assert.Equal(t, chaos.EventSchema, ev.Schema)
	assert.Equal(t, "r1", ev.RunID)
	assert.Equal(t, "netem delay", ev.Action)
	assert.Equal(t, &chaos.RunStart{
		Runtime:  "docker",
		DryRun:   true,
		Interval: "0s",
		Selection: chaos.Selection{
			Names: []string{"web-1"}, Labels: []string{}, MaxTargets: 2,
		},
		Params: map[string]any{
			"duration": "30s", "interface": "eth0", "target": []string{}, "pull-image": true,
			"time": 100, "correlation": 2.5,
		},
	}, ev.Start, "params hold typed action and parent flags; never globals or excluded flags")
}

func TestNewActionUsageErrors(t *testing.T) {
	parseErr := errors.New("unset or invalid duration value")
	buildErr := errors.New("invalid loss probability")
	tests := []struct {
		name    string
		spec    Spec[testParams]
		args    []string
		wantMsg string
	}{
		{
			name:    "missing container argument",
			spec:    Spec[testParams]{Name: "delay", RequireArgs: true},
			args:    []string{"netem", "delay"},
			wantMsg: ErrContainerArgRequired.Error(),
		},
		{
			name:    "invalid run id",
			spec:    Spec[testParams]{Name: "delay"},
			args:    []string{"--run-id", "bad id", "netem", "delay", "c1"},
			wantMsg: `invalid --run-id "bad id": use 1-48 characters from [A-Za-z0-9_.-]`,
		},
		{
			name: "parse error",
			spec: Spec[testParams]{Name: "delay", Parse: func(cliflags.Flags, *chaos.GlobalParams) (testParams, error) {
				return testParams{}, parseErr
			}},
			args:    []string{"netem", "delay", "c1"},
			wantMsg: parseErr.Error(),
		},
		{
			name: "build error",
			spec: Spec[testParams]{
				Name:  "delay",
				Parse: func(cliflags.Flags, *chaos.GlobalParams) (testParams, error) { return testParams{}, nil },
				Build: func(container.Client, *chaos.GlobalParams, testParams) (chaos.Command, error) { return nil, buildErr },
			},
			args:    []string{"netem", "delay", "c1"},
			wantMsg: buildErr.Error(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := &recordSink{}
			rt, _ := runtimeReturning(nil)
			err := runApp(t, NewAction(context.Background(), rt, sink, tt.spec), tt.args...)
			require.EqualError(t, err, tt.wantMsg, "message unchanged")
			o := chaos.NewOutcome(chaos.Counts{}, err, false)
			assert.Equal(t, chaos.RunUsageError, o.Status)
			assert.Equal(t, chaos.CodeInvalidArgument, o.Error.Code)
			assert.Equal(t, 1, o.ExitCode)
			assert.Empty(t, sink.events, "no run.start before the command is built")
		})
	}
}
