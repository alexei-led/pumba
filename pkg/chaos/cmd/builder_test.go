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
	cmd := NewAction(context.Background(), rt, spec)
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
	cmd := NewAction(context.Background(), rt, spec)
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
	cmd := NewAction(context.Background(), rt, spec)
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
	cmd := NewAction(context.Background(), rt, spec)
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
	cmd := NewAction(context.Background(), rt, spec)
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
	cmd := NewAction(context.Background(), rt, spec)
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
	cmd := NewAction(context.Background(), rt, spec)
	err := actionFunc(t, cmd)(newTestCLIContext([]string{"foo"}))
	require.Error(t, err)
	assert.ErrorIs(t, err, runErr)
	assert.Contains(t, err.Error(), "ploop", "wrapped error should mention command name")
}
