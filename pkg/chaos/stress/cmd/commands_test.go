package cmd

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
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

func newTestCLIContext(t *testing.T, flags []cli.Flag, args []string) *cli.Context {
	t.Helper()
	app := cli.NewApp()
	fs := flag.NewFlagSet("stress", flag.ContinueOnError)
	for _, f := range flags {
		f.Apply(fs)
	}
	require.NoError(t, fs.Parse(args))
	return cli.NewContext(app, fs, nil)
}

func TestNewStressCLICommand_Contract(t *testing.T) {
	rt, _, calls := fakeRuntime(t)
	cmd := NewStressCLICommand(context.Background(), rt)
	require.NotNil(t, cmd)
	assert.Equal(t, "stress", cmd.Name)
	assert.NotNil(t, cmd.Action)
	assert.Equal(t, 0, *calls, "Runtime must not be resolved at construction time")
}

func TestNewStressCLICommand_AcceptsNilRuntime(t *testing.T) {
	assert.NotNil(t, NewStressCLICommand(context.Background(), nilRuntime()))
}

func TestParseStressParams(t *testing.T) {
	cmd := NewStressCLICommand(context.Background(), nilRuntime())
	c := newTestCLIContext(t, cmd.Flags,
		[]string{"--duration", "30s", "--stress-image", "custom:tag", "--stressors", "--cpu 2"})
	got, err := parseStressParams(c, defaultGlobalParams())
	require.NoError(t, err)
	assert.Equal(t, "custom:tag", got.Image)
	assert.True(t, got.Pull, "BoolT pull-image defaults true")
	assert.Equal(t, "--cpu 2", got.Stressors)
	assert.Equal(t, 30*time.Second, got.Duration)
	assert.False(t, got.InjectCgroup)
}

func TestParseStressParams_InjectCgroup(t *testing.T) {
	cmd := NewStressCLICommand(context.Background(), nilRuntime())
	c := newTestCLIContext(t, cmd.Flags,
		[]string{"--duration", "10s", "--inject-cgroup"})
	got, err := parseStressParams(c, defaultGlobalParams())
	require.NoError(t, err)
	assert.True(t, got.InjectCgroup)
}

func TestParseStressParams_MissingDurationErrors(t *testing.T) {
	cmd := NewStressCLICommand(context.Background(), nilRuntime())
	c := newTestCLIContext(t, cmd.Flags, nil)
	_, err := parseStressParams(c, defaultGlobalParams())
	assert.ErrorContains(t, err, "duration")
}

func TestBuildStressCommand(t *testing.T) {
	client := container.NewMockClient(t)
	cmd := NewStressCLICommand(context.Background(), nilRuntime())
	c := newTestCLIContext(t, cmd.Flags, []string{"--duration", "10s"})
	p, err := parseStressParams(c, defaultGlobalParams())
	require.NoError(t, err)
	built, err := buildStressCommand(client, defaultGlobalParams(), p)
	require.NoError(t, err)
	require.NotNil(t, built)
}
