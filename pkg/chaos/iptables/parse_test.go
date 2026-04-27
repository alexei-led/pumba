package iptables

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
		cli.IntFlag{Name: "limit"},
	}
}

func parentCtx(t *testing.T, args []string) *cli.Context {
	t.Helper()
	app := cli.NewApp()
	fs := flag.NewFlagSet("iptables", flag.ContinueOnError)
	for _, f := range iptablesParentFlags() {
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
		wantProt string
		wantDur  time.Duration
	}{
		{
			name:     "happy path with defaults",
			args:     []string{"--duration", "10s", "--interface", "eth0", "--limit", "3"},
			gp:       &chaos.GlobalParams{},
			wantLim:  3,
			wantIfac: "eth0",
			wantProt: "any",
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
			name:    "bad protocol rejected",
			args:    []string{"--duration", "1s", "--interface", "eth0", "--protocol", "sctp"},
			gp:      &chaos.GlobalParams{},
			wantErr: "bad protocol name",
		},
		{
			name:    "invalid CIDR (source) rejected",
			args:    []string{"--duration", "1s", "--interface", "eth0", "--source", "not-a-cidr"},
			gp:      &chaos.GlobalParams{},
			wantErr: "failed to parse ip",
		},
		{
			name:    "invalid CIDR (destination) rejected",
			args:    []string{"--duration", "1s", "--interface", "eth0", "--destination", "not-a-cidr"},
			gp:      &chaos.GlobalParams{},
			wantErr: "failed to parse ip",
		},
		{
			name:    "invalid src port rejected",
			args:    []string{"--duration", "1s", "--interface", "eth0", "--protocol", "tcp", "--src-port", "abc"},
			gp:      &chaos.GlobalParams{},
			wantErr: "failed to get source ports",
		},
		{
			name:    "invalid dst port rejected",
			args:    []string{"--duration", "1s", "--interface", "eth0", "--protocol", "tcp", "--dst-port", "abc"},
			gp:      &chaos.GlobalParams{},
			wantErr: "failed to get destination ports",
		},
		{
			name:    "src port with non-tcp/udp rejected",
			args:    []string{"--duration", "1s", "--interface", "eth0", "--protocol", "icmp", "--src-port", "80"},
			gp:      &chaos.GlobalParams{},
			wantErr: "using source port is only supported",
		},
		{
			name:    "dst port with non-tcp/udp rejected",
			args:    []string{"--duration", "1s", "--interface", "eth0", "--protocol", "icmp", "--dst-port", "80"},
			gp:      &chaos.GlobalParams{},
			wantErr: "using destination port is only supported",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cliflags.NewV1(parentCtx(t, tt.args))
			base, err := ParseRequestBase(c, tt.gp)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, base)
			require.NotNil(t, base.Request)
			assert.Equal(t, tt.wantLim, base.Limit)
			assert.Equal(t, tt.wantIfac, base.Iface)
			assert.Equal(t, tt.wantProt, base.Protocol)
			assert.Equal(t, tt.wantDur, base.Request.Duration)
			assert.Empty(t, base.Request.CmdPrefix, "CmdPrefix must be left for per-action parser to fill")
			assert.Empty(t, base.Request.CmdSuffix, "CmdSuffix must be left for per-action parser to fill")
			assert.Nil(t, base.Request.Container, "Container must be left for per-iteration assignment")
		})
	}
}

func TestParseRequestBase_DryRunPropagates(t *testing.T) {
	c := cliflags.NewV1(parentCtx(t, []string{"--duration", "1s", "--interface", "eth0"}))
	base, err := ParseRequestBase(c, &chaos.GlobalParams{DryRun: true})
	require.NoError(t, err)
	assert.True(t, base.Request.DryRun)
}

func TestParseRequestBase_PortsAndIPsParsed(t *testing.T) {
	c := cliflags.NewV1(parentCtx(t, []string{
		"--duration", "1s", "--interface", "eth0", "--protocol", "tcp",
		"--source", "10.0.0.0/24",
		"--destination", "10.1.0.0/16",
		"--src-port", "80,443",
		"--dst-port", "8080",
	}))
	base, err := ParseRequestBase(c, &chaos.GlobalParams{})
	require.NoError(t, err)
	require.Len(t, base.Request.SrcIPs, 1)
	assert.Equal(t, "10.0.0.0/24", base.Request.SrcIPs[0].String())
	require.Len(t, base.Request.DstIPs, 1)
	assert.Equal(t, "10.1.0.0/16", base.Request.DstIPs[0].String())
	assert.Equal(t, []string{"80", "443"}, base.Request.SPorts)
	assert.Equal(t, []string{"8080"}, base.Request.DPorts)
}
