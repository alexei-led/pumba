package cmd

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/alexei-led/pumba/pkg/chaos/status"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func parse(t *testing.T, gp *chaos.GlobalParams, args ...string) (status.Params, error) {
	t.Helper()
	cmd := NewStatusCLICommand(context.Background(), func() container.Client { return nil }, chaos.NopSink{})
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	for _, f := range cmd.Flags {
		f.Apply(fs)
	}
	require.NoError(t, fs.Parse(args))
	return parseStatusParams(cliflags.NewV1(cli.NewContext(cli.NewApp(), fs, nil)), gp)
}

func TestParseStatusParams(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		interval time.Duration
		want     status.Params
		wantErr  string
	}{
		{
			name: "defaults",
			want: status.Params{Sidecar: container.SidecarSpec{Image: "ghcr.io/alexei-led/pumba-alpine-nettools:latest", Pull: true}},
		},
		{
			name: "run filter, check and image",
			args: []string{"--run-id", "run-1", "--check", "--tc-image", "img", "--pull-image=false"},
			want: status.Params{RunFilter: "run-1", Check: true, Sidecar: container.SidecarSpec{Image: "img"}},
		},
		{
			name:     "interval without check",
			interval: time.Minute,
			want:     status.Params{Sidecar: container.SidecarSpec{Image: "ghcr.io/alexei-led/pumba-alpine-nettools:latest", Pull: true}},
		},
		{
			name:     "check rejects interval",
			args:     []string{"--check"},
			interval: time.Minute,
			wantErr:  "--check cannot be combined with --interval",
		},
		{
			name:    "invalid run filter",
			args:    []string{"--run-id", "a b"},
			wantErr: `invalid --run-id "a b"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parse(t, &chaos.GlobalParams{Interval: tt.interval}, tt.args...)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewStatusCLICommandDoesNotResolveRuntime(t *testing.T) {
	calls := 0
	cmd := NewStatusCLICommand(context.Background(), func() container.Client { calls++; return nil }, chaos.NopSink{})
	assert.Equal(t, "status", cmd.Name)
	assert.Zero(t, calls)
}
