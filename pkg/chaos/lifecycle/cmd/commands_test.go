package cmd

import (
	"context"
	"flag"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli"
)

func newTestCLIContext(args []string) *cli.Context {
	app := cli.NewApp()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = fs.Parse(args)
	return cli.NewContext(app, fs, nil)
}

func nilRuntime() chaos.Runtime {
	return func() container.Client { return nil }
}

// fakeRuntime returns a Runtime closure plus a pointer that records each invocation,
// so tests can assert the constructor stored the closure (call count) and that the
// closure resolves to the injected client.
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

func TestKillRequiresContainerArgument(t *testing.T) {
	cmd := &killContext{context: context.Background()}
	c := newTestCLIContext(nil)
	err := cmd.kill(c)
	assert.EqualError(t, err, "container name, list of names, or RE2 regex is required")
}

func TestStopRequiresContainerArgument(t *testing.T) {
	cmd := &stopContext{context: context.Background()}
	c := newTestCLIContext(nil)
	err := cmd.stop(c)
	assert.EqualError(t, err, "container name, list of names, or RE2 regex is required")
}

func TestRemoveRequiresContainerArgument(t *testing.T) {
	cmd := &removeContext{context: context.Background()}
	c := newTestCLIContext(nil)
	err := cmd.remove(c)
	assert.EqualError(t, err, "container name, list of names, or RE2 regex is required")
}

func TestNewKillCLICommand_StoresRuntime(t *testing.T) {
	rt, want, _ := fakeRuntime(t)
	cli := NewKillCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "kill", cli.Name)
	cmdCtx := &killContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

func TestNewStopCLICommand_StoresRuntime(t *testing.T) {
	rt, want, _ := fakeRuntime(t)
	cli := NewStopCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "stop", cli.Name)
	cmdCtx := &stopContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

func TestNewRemoveCLICommand_StoresRuntime(t *testing.T) {
	rt, want, _ := fakeRuntime(t)
	cli := NewRemoveCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "rm", cli.Name)
	cmdCtx := &removeContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

func TestNewExecCLICommand_StoresRuntime(t *testing.T) {
	rt, want, _ := fakeRuntime(t)
	cli := NewExecCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "exec", cli.Name)
	cmdCtx := &execContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

func TestNewPauseCLICommand_StoresRuntime(t *testing.T) {
	rt, want, _ := fakeRuntime(t)
	cli := NewPauseCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "pause", cli.Name)
	cmdCtx := &pauseContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

func TestNewRestartCLICommand_StoresRuntime(t *testing.T) {
	rt, want, _ := fakeRuntime(t)
	cli := NewRestartCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "restart", cli.Name)
	cmdCtx := &restartContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

// TestRuntimeAcceptsNil verifies the constructors accept a nil-returning Runtime
// without panicking — important since main.go's runtime closure can return nil
// before before() has constructed a client (e.g. in --help/--version paths).
func TestRuntimeAcceptsNil(t *testing.T) {
	rt := nilRuntime()
	assert.NotNil(t, NewKillCLICommand(context.Background(), rt))
	assert.NotNil(t, NewStopCLICommand(context.Background(), rt))
	assert.NotNil(t, NewRemoveCLICommand(context.Background(), rt))
	assert.NotNil(t, NewExecCLICommand(context.Background(), rt))
	assert.NotNil(t, NewPauseCLICommand(context.Background(), rt))
	assert.NotNil(t, NewRestartCLICommand(context.Background(), rt))
}
