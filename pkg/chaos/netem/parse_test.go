package netem

import (
	"flag"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func netemParentFlags() []cli.Flag {
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

func parentCtx(t *testing.T, args []string) *cli.Context {
	t.Helper()
	app := cli.NewApp()
	fs := flag.NewFlagSet("netem", flag.ContinueOnError)
	for _, f := range netemParentFlags() {
		f.Apply(fs)
	}
	require.NoError(t, fs.Parse(args))
	return cli.NewContext(app, fs, nil)
}

func TestParseRequestBase(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		gp       *chaos.GlobalParams
		wantErr  string
		wantLim  int
		wantIfac string
		wantDur  time.Duration
	}{
		{
			name:     "happy path with defaults",
			args:     []string{"--duration", "10s", "--interface", "eth0", "--limit", "3"},
			gp:       &chaos.GlobalParams{},
			wantLim:  3,
			wantIfac: "eth0",
			wantDur:  10 * time.Second,
		},
		{
			name:    "zero duration rejected",
			args:    []string{"--interface", "eth0"},
			gp:      &chaos.GlobalParams{},
			wantErr: "unset or invalid duration value",
		},
		{
			name:    "duration not shorter than interval rejected",
			args:    []string{"--duration", "5s", "--interface", "eth0"},
			gp:      &chaos.GlobalParams{Interval: 5 * time.Second},
			wantErr: "duration must be shorter than interval",
		},
		{
			name:    "bad interface rejected",
			args:    []string{"--duration", "1s", "--interface", "1bad;rm"},
			gp:      &chaos.GlobalParams{},
			wantErr: "bad network interface name",
		},
		{
			name:    "invalid CIDR rejected",
			args:    []string{"--duration", "1s", "--interface", "eth0", "--target", "not-a-cidr"},
			gp:      &chaos.GlobalParams{},
			wantErr: "failed to parse ip",
		},
		{
			name:    "invalid egress port rejected",
			args:    []string{"--duration", "1s", "--interface", "eth0", "--egress-port", "abc"},
			gp:      &chaos.GlobalParams{},
			wantErr: "failed to get source ports",
		},
		{
			name:    "invalid ingress port rejected",
			args:    []string{"--duration", "1s", "--interface", "eth0", "--ingress-port", "abc"},
			gp:      &chaos.GlobalParams{},
			wantErr: "failed to get destination ports",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cliflags.NewV1(parentCtx(t, tt.args))
			req, limit, err := ParseRequestBase(c, tt.gp)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, req)
			assert.Equal(t, tt.wantLim, limit)
			assert.Equal(t, tt.wantIfac, req.Interface)
			assert.Equal(t, tt.wantDur, req.Duration)
			assert.Empty(t, req.Command, "Command must be left for per-action parser to fill")
			assert.Nil(t, req.Container, "Container must be left for per-iteration assignment")
		})
	}
}

func TestParseRequestBase_DryRunPropagates(t *testing.T) {
	c := cliflags.NewV1(parentCtx(t, []string{"--duration", "1s", "--interface", "eth0"}))
	req, _, err := ParseRequestBase(c, &chaos.GlobalParams{DryRun: true})
	require.NoError(t, err)
	assert.True(t, req.DryRun)
}

func TestParseRequestBase_PortsAndIPsParsed(t *testing.T) {
	c := cliflags.NewV1(parentCtx(t, []string{
		"--duration", "1s", "--interface", "eth0",
		"--target", "10.0.0.0/24",
		"--egress-port", "80,443",
		"--ingress-port", "8080",
	}))
	req, _, err := ParseRequestBase(c, &chaos.GlobalParams{})
	require.NoError(t, err)
	require.Len(t, req.IPs, 1)
	assert.Equal(t, "10.0.0.0/24", req.IPs[0].String())
	assert.Equal(t, []string{"80", "443"}, req.SPorts)
	assert.Equal(t, []string{"8080"}, req.DPorts)
}
