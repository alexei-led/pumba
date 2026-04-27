package cliflags_test

import (
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli"
)

// TestNewV1FromApp_GlobalFlagReads exercises every Flags read method against
// the app-level cli.Context that cmd/main.go passes to before() and
// createRuntimeClient(). Mirrors the global-flag set declared in cmd/main.go
// (runtime, host, log-level, json, slackhook, slackchannel, tls* triplet,
// label, interval) so a regression in the adapter surface fails here.
func TestNewV1FromApp_GlobalFlagReads(t *testing.T) {
	app := newCtx(t,
		[]cli.Flag{
			cli.StringFlag{Name: "runtime"},
			cli.StringFlag{Name: "host"},
			cli.StringFlag{Name: "log-level"},
			cli.BoolFlag{Name: "json"},
			cli.StringFlag{Name: "slackhook"},
			cli.StringFlag{Name: "slackchannel"},
			cli.BoolFlag{Name: "tlsverify"},
			cli.StringFlag{Name: "tlscacert"},
			cli.DurationFlag{Name: "interval"},
			cli.IntFlag{Name: "limit"},
			cli.Float64Flag{Name: "ratio"},
			cli.StringSliceFlag{Name: "label"},
		},
		[]string{
			"--runtime", "podman",
			"--host", "unix:///var/run/docker.sock",
			"--log-level", "debug",
			"--json",
			"--slackhook", "https://hooks.slack.example/svc",
			"--slackchannel", "#chaos",
			"--tlsverify",
			"--tlscacert", "/etc/ssl/docker/ca.pem",
			"--interval", "30s",
			"--limit", "5",
			"--ratio", "0.5",
			"--label", "team=sre",
			"--label", "env=prod",
		},
	)

	f := cliflags.NewV1FromApp(app)

	assert.Equal(t, "podman", f.String("runtime"))
	assert.Equal(t, "unix:///var/run/docker.sock", f.String("host"))
	assert.Equal(t, "debug", f.String("log-level"))
	assert.True(t, f.Bool("json"))
	assert.Equal(t, "https://hooks.slack.example/svc", f.String("slackhook"))
	assert.Equal(t, "#chaos", f.String("slackchannel"))
	assert.True(t, f.Bool("tlsverify"))
	assert.Equal(t, "/etc/ssl/docker/ca.pem", f.String("tlscacert"))
	assert.Equal(t, 30*time.Second, f.Duration("interval"))
	assert.Equal(t, 5, f.Int("limit"))
	assert.InEpsilon(t, 0.5, f.Float64("ratio"), 1e-9)
	assert.Equal(t, []string{"team=sre", "env=prod"}, f.StringSlice("label"))
}

// TestNewV1FromApp_NilParent confirms the root-context contract: a Flags
// returned from NewV1FromApp has no parent. before() relies on this when it
// short-circuits the Parent()-walking logic in subcommand parsers.
func TestNewV1FromApp_NilParent(t *testing.T) {
	c := newCtx(t, nil, nil)
	f := cliflags.NewV1FromApp(c)
	assert.Nil(t, f.Parent(), "app-level Flags has no parent")
}

// TestNewV1FromApp_GlobalReturnsSelf documents the equivalence stated in the
// constructor godoc — Global() on an app-level Flags is the identity for the
// reads that matter. Lets callers replace NewV1(c).Global() with NewV1FromApp(c)
// without behavior change.
func TestNewV1FromApp_GlobalReturnsSelf(t *testing.T) {
	c := newCtx(t,
		[]cli.Flag{cli.StringFlag{Name: "log-level"}},
		[]string{"--log-level", "warning"},
	)
	f := cliflags.NewV1FromApp(c)
	assert.Equal(t, "warning", f.Global().String("log-level"))
}
