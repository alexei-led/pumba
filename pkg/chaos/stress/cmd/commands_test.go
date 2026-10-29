package cmd

import (
	"context"
	"flag"
	"strings"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	got, err := parseStressParams(cliflags.NewV1(c), defaultGlobalParams())
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
	got, err := parseStressParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	assert.True(t, got.InjectCgroup)
}

func TestParseStressParams_MissingDurationErrors(t *testing.T) {
	cmd := NewStressCLICommand(context.Background(), nilRuntime())
	c := newTestCLIContext(t, cmd.Flags, nil)
	_, err := parseStressParams(cliflags.NewV1(c), defaultGlobalParams())
	assert.ErrorContains(t, err, "duration")
}

func TestParseStressParams_NegativeDurationErrors(t *testing.T) {
	cmd := NewStressCLICommand(context.Background(), nilRuntime())
	c := newTestCLIContext(t, cmd.Flags, []string{"--duration", "-1s"})
	_, err := parseStressParams(cliflags.NewV1(c), defaultGlobalParams())
	assert.EqualError(t, err, "unset or invalid duration value")
}

func TestBuildStressCommand(t *testing.T) {
	client := container.NewMockClient(t)
	cmd := NewStressCLICommand(context.Background(), nilRuntime())
	c := newTestCLIContext(t, cmd.Flags, []string{"--duration", "10s"})
	p, err := parseStressParams(cliflags.NewV1(c), defaultGlobalParams())
	require.NoError(t, err)
	built, err := buildStressCommand(client, defaultGlobalParams(), p)
	require.NoError(t, err)
	require.NotNil(t, built)
}

func TestParseStressParams_FirstClass(t *testing.T) {
	for _, tt := range []struct {
		name          string
		args          []string
		want, wantErr string
	}{
		{name: "legacy default", want: "--cpu 4 --timeout 60s"},
		{name: "legacy raw", args: []string{"--stressors=--vm 0 --timeout 5s"}, want: "--vm 0 --timeout 5s"},
		{name: "cpu", args: []string{"--cpu", "2", "--cpu-load", "80"}, want: "--cpu 2 --cpu-load 80 --timeout 30s"},
		{name: "zero load", args: []string{"--cpu", "1", "--cpu-load", "0"}, want: "--cpu 1 --cpu-load 0 --timeout 30s"},
		{name: "full load", args: []string{"--cpu", "1", "--cpu-load", "100"}, want: "--cpu 1 --cpu-load 100 --timeout 30s"},
		{name: "memory bytes", args: []string{"--memory", "2", "--memory-bytes", "256M"}, want: "--vm 2 --vm-bytes 256M --timeout 30s"},
		{name: "memory percent", args: []string{"--memory", "1", "--memory-bytes", "75%"}, want: "--vm 1 --vm-bytes 75% --timeout 30s"},
		{name: "percent lower", args: []string{"--memory", "1", "--memory-bytes", "1%"}, want: "--vm 1 --vm-bytes 1% --timeout 30s"},
		{name: "percent upper", args: []string{"--memory", "1", "--memory-bytes", "100%"}, want: "--vm 1 --vm-bytes 100% --timeout 30s"},
		{name: "combined", args: []string{"--cpu", "2", "--memory", "1", "--io", "3"}, want: "--cpu 2 --vm 1 --io 3 --timeout 30s"},
		{name: "fractional duration", args: []string{"--duration", "1500ms", "--io", "1"}, want: "--io 1 --timeout 2s"},
		{name: "tiny duration", args: []string{"--duration", "1ns", "--cpu", "1"}, want: "--cpu 1 --timeout 1s"},
		{name: "raw conflict", args: []string{"--cpu", "1", "--stressors=--io 2"}, wantErr: "cannot combine --stressors"},
		{name: "explicit default conflict", args: []string{"--io", "1", "--stressors=--cpu 4 --timeout 60s"}, wantErr: "cannot combine --stressors"},
		{name: "empty raw conflict", args: []string{"--io", "1", "--stressors="}, wantErr: "cannot combine --stressors"},
		{name: "zero workers", args: []string{"--cpu", "0"}, wantErr: "--cpu must be at least 1"},
		{name: "negative workers", args: []string{"--memory", "-1"}, wantErr: "--memory must be at least 1"},
		{name: "zero io", args: []string{"--io", "0"}, wantErr: "--io must be at least 1"},
		{name: "negative load", args: []string{"--cpu", "1", "--cpu-load", "-1"}, wantErr: "--cpu-load must be between 0 and 100"},
		{name: "excess load", args: []string{"--cpu", "1", "--cpu-load", "101"}, wantErr: "--cpu-load must be between 0 and 100"},
		{name: "load needs cpu", args: []string{"--cpu-load", "80"}, wantErr: "--cpu-load requires --cpu"},
		{name: "bytes need memory", args: []string{"--memory-bytes", "256M"}, wantErr: "--memory-bytes requires --memory"},
		{name: "zero percent", args: []string{"--memory", "1", "--memory-bytes", "0%"}, wantErr: "--memory-bytes percent must be between 1 and 100"},
		{name: "excess percent", args: []string{"--memory", "1", "--memory-bytes", "101%"}, wantErr: "--memory-bytes percent must be between 1 and 100"},
		{name: "invalid bytes", args: []string{"--memory", "1", "--memory-bytes", "256M --cpu 5"}, wantErr: "invalid --memory-bytes"},
		{name: "empty bytes", args: []string{"--memory", "1", "--memory-bytes", ""}, wantErr: "invalid --memory-bytes"},
		{name: "zero bytes", args: []string{"--memory", "1", "--memory-bytes", "0"}, wantErr: "invalid --memory-bytes"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewStressCLICommand(context.Background(), nilRuntime())
			args := append([]string{"--duration", "30s"}, tt.args...)
			c := newTestCLIContext(t, cmd.Flags, args)
			got, err := parseStressParams(cliflags.NewV1(c), defaultGlobalParams())
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, strings.Fields(tt.want), strings.Fields(got.Stressors))
		})
	}
}

func TestStressCLI_ResourceNamesRemainTargets(t *testing.T) {
	for _, name := range []string{"cpu", "memory", "io"} {
		for _, args := range [][]string{{"--cpu", "2"}, {"--stressors=--cpu 2 --timeout 5s"}} {
			t.Run(name+strings.Join(args, " "), func(t *testing.T) {
				client := container.NewMockClient(t)
				target := &container.Container{ContainerID: name, ContainerName: name}
				client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).RunAndReturn(func(_ context.Context, filter container.FilterFunc, _ container.ListOpts) ([]*container.Container, error) {
					assert.True(t, filter(target))
					assert.False(t, filter(&container.Container{ContainerName: "unselected"}))
					return []*container.Container{target}, nil
				}).Once()
				stressors := []string{"--cpu", "2", "--timeout", "60s"}
				if strings.HasPrefix(args[0], "--stressors") {
					stressors = []string{"--cpu", "2", "--timeout", "5s"}
				}
				client.EXPECT().StressContainer(mock.Anything, &container.StressRequest{
					Container: target, Stressors: stressors, Duration: time.Minute, DryRun: true,
					Sidecar: container.SidecarSpec{Image: "ghcr.io/alexei-led/stress-ng:latest", Pull: true},
				}).Return(&container.StressResult{}, nil).Once()
				app := cli.NewApp()
				app.Flags = []cli.Flag{cli.BoolFlag{Name: "dry-run"}}
				app.Commands = []cli.Command{*NewStressCLICommand(context.Background(), func() container.Client { return client })}
				argv := append([]string{"pumba", "--dry-run", "stress", "--duration", "1m"}, args...)
				require.NoError(t, app.Run(append(argv, name)))
			})
		}
	}
}
