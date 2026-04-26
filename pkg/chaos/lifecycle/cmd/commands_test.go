package cmd

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

// newTestCLIContext builds a *cli.Context whose flag set carries the given
// flags and parsed values. Pass `args` like `[]string{"--signal", "SIGTERM"}`.
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

// fakeRuntime returns a Runtime that resolves to a real MockClient and a
// pointer to a counter so tests can assert how many times the runtime closure
// was invoked.
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

// findFlag locates a registered flag on a *cli.Command by primary name
// (matches against the head of "name, alias"). Used to drive parser tests
// without rebuilding flag definitions inline.
func findFlag(t *testing.T, cmd *cli.Command, name string) cli.Flag {
	t.Helper()
	for _, f := range cmd.Flags {
		if f.GetName() == name || splitFirst(f.GetName()) == name {
			return f
		}
	}
	t.Fatalf("flag %q not registered on command %q", name, cmd.Name)
	return nil
}

func splitFirst(name string) string {
	for i, ch := range name {
		if ch == ',' {
			return name[:i]
		}
	}
	return name
}

// constructorContract bundles the per-command facts every constructor must
// satisfy: name, that an Action closure was attached, that a Runtime can be
// stored and never resolved before the action fires.
func assertConstructorContract(t *testing.T, cmd *cli.Command, wantName string) {
	t.Helper()
	require.NotNil(t, cmd, "constructor returned nil command")
	assert.Equal(t, wantName, cmd.Name)
	assert.NotNil(t, cmd.Action, "constructor must wire an Action closure")
}

// ---- Kill ----------------------------------------------------------------

func TestNewKillCLICommand_Contract(t *testing.T) {
	rt, _, calls := fakeRuntime(t)
	cmd := NewKillCLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "kill")
	assert.Equal(t, 0, *calls, "Runtime must not be resolved at construction time")
}

func TestParseKillParams(t *testing.T) {
	cmd := NewKillCLICommand(context.Background(), nilRuntime())
	c := newTestCLIContext(t, []cli.Flag{findFlag(t, cmd, "signal"), findFlag(t, cmd, "limit")},
		[]string{"--signal", "SIGTERM", "--limit", "3"})
	got, err := parseKillParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.Equal(t, KillParams{Signal: "SIGTERM", Limit: 3}, got)
}

func TestBuildKillCommand_ReturnsLifecycleKill(t *testing.T) {
	client := container.NewMockClient(t)
	cmd, err := buildKillCommand(client, defaultGlobalParams(), KillParams{Signal: "SIGTERM", Limit: 1})
	require.NoError(t, err)
	require.NotNil(t, cmd)
}

func TestBuildKillCommand_RejectsInvalidSignal(t *testing.T) {
	client := container.NewMockClient(t)
	_, err := buildKillCommand(client, defaultGlobalParams(), KillParams{Signal: "NOT-A-SIGNAL"})
	assert.Error(t, err)
}

// ---- Stop ----------------------------------------------------------------

func TestNewStopCLICommand_Contract(t *testing.T) {
	rt, _, calls := fakeRuntime(t)
	cmd := NewStopCLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "stop")
	assert.Equal(t, 0, *calls)
}

func TestParseStopParams(t *testing.T) {
	cmd := NewStopCLICommand(context.Background(), nilRuntime())
	flags := []cli.Flag{
		findFlag(t, cmd, "time"), findFlag(t, cmd, "limit"),
		findFlag(t, cmd, "restart"), findFlag(t, cmd, "duration"),
	}
	c := newTestCLIContext(t, flags,
		[]string{"--time", "5", "--limit", "2", "--restart", "--duration", "10s"})
	got, err := parseStopParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.Equal(t, StopParams{WaitTime: 5, Limit: 2, Restart: true, Duration: 10 * time.Second}, got)
}

func TestParseStopParams_InvalidDurationErrors(t *testing.T) {
	cmd := NewStopCLICommand(context.Background(), nilRuntime())
	flags := []cli.Flag{
		findFlag(t, cmd, "time"), findFlag(t, cmd, "limit"),
		findFlag(t, cmd, "restart"), findFlag(t, cmd, "duration"),
	}
	// Stop's --duration carries a default of "10s", so test the unparseable
	// path (non-empty, non-duration string) to exercise the zero-check.
	c := newTestCLIContext(t, flags, []string{"--duration", "not-a-duration"})
	_, err := parseStopParams(cliflags.NewV1(c), defaultGlobalParams())
	assert.EqualError(t, err, "unset or invalid duration value")
}

func TestBuildStopCommand_ReturnsLifecycleStop(t *testing.T) {
	client := container.NewMockClient(t)
	cmd, err := buildStopCommand(client, defaultGlobalParams(),
		StopParams{Restart: true, Duration: time.Second, WaitTime: 1, Limit: 0})
	require.NoError(t, err)
	require.NotNil(t, cmd)
}

// ---- Pause ---------------------------------------------------------------

func TestNewPauseCLICommand_Contract(t *testing.T) {
	rt, _, _ := fakeRuntime(t)
	cmd := NewPauseCLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "pause")
}

func TestParsePauseParams(t *testing.T) {
	cmd := NewPauseCLICommand(context.Background(), nilRuntime())
	flags := []cli.Flag{findFlag(t, cmd, "duration"), findFlag(t, cmd, "limit")}
	c := newTestCLIContext(t, flags, []string{"--duration", "2s", "--limit", "4"})
	got, err := parsePauseParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.Equal(t, PauseParams{Duration: 2 * time.Second, Limit: 4}, got)
}

func TestParsePauseParams_ZeroDurationErrors(t *testing.T) {
	cmd := NewPauseCLICommand(context.Background(), nilRuntime())
	flags := []cli.Flag{findFlag(t, cmd, "duration"), findFlag(t, cmd, "limit")}
	c := newTestCLIContext(t, flags, nil)
	_, err := parsePauseParams(cliflags.NewV1(c), defaultGlobalParams())
	assert.EqualError(t, err, "unset or invalid duration value")
}

func TestBuildPauseCommand(t *testing.T) {
	client := container.NewMockClient(t)
	cmd, err := buildPauseCommand(client, defaultGlobalParams(),
		PauseParams{Duration: time.Second, Limit: 0})
	require.NoError(t, err)
	require.NotNil(t, cmd)
}

// ---- Restart -------------------------------------------------------------

func TestNewRestartCLICommand_Contract(t *testing.T) {
	rt, _, _ := fakeRuntime(t)
	cmd := NewRestartCLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "restart")
}

func TestParseRestartParams(t *testing.T) {
	cmd := NewRestartCLICommand(context.Background(), nilRuntime())
	flags := []cli.Flag{findFlag(t, cmd, "timeout"), findFlag(t, cmd, "limit")}
	c := newTestCLIContext(t, flags, []string{"--timeout", "5s", "--limit", "1"})
	got, err := parseRestartParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.Equal(t, RestartParams{Timeout: 5 * time.Second, Limit: 1}, got)
}

func TestBuildRestartCommand(t *testing.T) {
	client := container.NewMockClient(t)
	cmd, err := buildRestartCommand(client, defaultGlobalParams(),
		RestartParams{Timeout: time.Second, Limit: 0})
	require.NoError(t, err)
	require.NotNil(t, cmd)
}

// ---- Remove --------------------------------------------------------------

func TestNewRemoveCLICommand_Contract(t *testing.T) {
	rt, _, _ := fakeRuntime(t)
	cmd := NewRemoveCLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "rm")
}

func TestParseRemoveParams(t *testing.T) {
	cmd := NewRemoveCLICommand(context.Background(), nilRuntime())
	flags := []cli.Flag{
		findFlag(t, cmd, "force"), findFlag(t, cmd, "links"),
		findFlag(t, cmd, "volumes"), findFlag(t, cmd, "limit"),
	}
	c := newTestCLIContext(t, flags, []string{"--limit", "7"})
	got, err := parseRemoveParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	// BoolTFlag defaults to true (force, volumes); BoolFlag defaults to false (links).
	assert.Equal(t, RemoveParams{Force: true, Links: false, Volumes: true, Limit: 7}, got)
}

func TestBuildRemoveCommand(t *testing.T) {
	client := container.NewMockClient(t)
	cmd, err := buildRemoveCommand(client, defaultGlobalParams(),
		RemoveParams{Force: true, Links: false, Volumes: true, Limit: 0})
	require.NoError(t, err)
	require.NotNil(t, cmd)
}

// ---- Exec ----------------------------------------------------------------

func TestNewExecCLICommand_Contract(t *testing.T) {
	rt, _, _ := fakeRuntime(t)
	cmd := NewExecCLICommand(context.Background(), rt)
	assertConstructorContract(t, cmd, "exec")
}

func TestParseExecParams(t *testing.T) {
	cmd := NewExecCLICommand(context.Background(), nilRuntime())
	flags := []cli.Flag{
		findFlag(t, cmd, "command"), findFlag(t, cmd, "args"), findFlag(t, cmd, "limit"),
	}
	c := newTestCLIContext(t, flags,
		[]string{"--command", "touch", "--args", "/tmp/a", "--args", "/tmp/b", "--limit", "2"})
	got, err := parseExecParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.Equal(t, ExecParams{Command: "touch", Args: []string{"/tmp/a", "/tmp/b"}, Limit: 2}, got)
}

func TestBuildExecCommand(t *testing.T) {
	client := container.NewMockClient(t)
	cmd, err := buildExecCommand(client, defaultGlobalParams(),
		ExecParams{Command: "echo", Args: []string{"hi"}, Limit: 0})
	require.NoError(t, err)
	require.NotNil(t, cmd)
}

// ---- Cross-cutting -------------------------------------------------------

// TestRuntimeAcceptsNil guards the same property as before: every constructor
// returns a usable *cli.Command even when the runtime closure happens to
// return nil (e.g. before main.go's before() has built a client during
// --help/--version paths).
func TestRuntimeAcceptsNil(t *testing.T) {
	rt := nilRuntime()
	assert.NotNil(t, NewKillCLICommand(context.Background(), rt))
	assert.NotNil(t, NewStopCLICommand(context.Background(), rt))
	assert.NotNil(t, NewRemoveCLICommand(context.Background(), rt))
	assert.NotNil(t, NewExecCLICommand(context.Background(), rt))
	assert.NotNil(t, NewPauseCLICommand(context.Background(), rt))
	assert.NotNil(t, NewRestartCLICommand(context.Background(), rt))
}

// TestKillRequiresContainerArgument verifies RequireArgs short-circuit on the
// kill builder: with no positional args the action returns the canonical error
// before ever resolving the runtime.
func TestKillRequiresContainerArgument(t *testing.T) {
	rt, _, calls := fakeRuntime(t)
	cmd := NewKillCLICommand(context.Background(), rt)
	action, ok := cmd.Action.(func(*cli.Context) error)
	require.True(t, ok)
	err := action(newTestCLIContext(t, cmd.Flags, nil))
	assert.EqualError(t, err, "container name, list of names, or RE2 regex is required")
	assert.Equal(t, 0, *calls)
}

func TestRemoveRequiresContainerArgument(t *testing.T) {
	rt, _, calls := fakeRuntime(t)
	cmd := NewRemoveCLICommand(context.Background(), rt)
	action, ok := cmd.Action.(func(*cli.Context) error)
	require.True(t, ok)
	err := action(newTestCLIContext(t, cmd.Flags, nil))
	assert.EqualError(t, err, "container name, list of names, or RE2 regex is required")
	assert.Equal(t, 0, *calls)
}

// Stop now enforces RequireArgs at the action level. Verify that no positional
// arg short-circuits to the canonical error before parsing runs.
func TestStopRequiresContainerArgument(t *testing.T) {
	rt, _, calls := fakeRuntime(t)
	cmd := NewStopCLICommand(context.Background(), rt)
	action, ok := cmd.Action.(func(*cli.Context) error)
	require.True(t, ok)
	err := action(newTestCLIContext(t, cmd.Flags, nil))
	assert.EqualError(t, err, "container name, list of names, or RE2 regex is required")
	assert.Equal(t, 0, *calls)
}
