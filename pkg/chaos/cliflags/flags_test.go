package cliflags_test

import (
	"flag"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

// newCtx builds a populated *cli.Context with the given flags + args. Tests
// drive the V1 adapter through it.
func newCtx(t *testing.T, flags []cli.Flag, args []string) *cli.Context {
	t.Helper()
	app := cli.NewApp()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, fl := range flags {
		fl.Apply(fs)
	}
	require.NoError(t, fs.Parse(args))
	return cli.NewContext(app, fs, nil)
}

// childCtx mounts subFlags on a child flag set whose parent is `parent`.
// Mirrors how urfave/cli v1 hands subcommand parsers a *cli.Context with a
// linked parent.
func childCtx(t *testing.T, parent *cli.Context, subFlags []cli.Flag, args []string) *cli.Context {
	t.Helper()
	fs := flag.NewFlagSet("sub", flag.ContinueOnError)
	for _, fl := range subFlags {
		fl.Apply(fs)
	}
	require.NoError(t, fs.Parse(args))
	return cli.NewContext(parent.App, fs, parent)
}

func TestV1_StringBoolBoolT(t *testing.T) {
	c := newCtx(t,
		[]cli.Flag{
			cli.StringFlag{Name: "name"},
			cli.BoolFlag{Name: "verbose"},
			cli.BoolTFlag{Name: "color"},
		},
		[]string{"--name", "alice", "--verbose"},
	)
	f := cliflags.NewV1(c)
	assert.Equal(t, "alice", f.String("name"))
	assert.True(t, f.Bool("verbose"))
	assert.True(t, f.BoolT("color"), "BoolT defaults to true")
}

func TestV1_DurationIntFloat64(t *testing.T) {
	c := newCtx(t,
		[]cli.Flag{
			cli.DurationFlag{Name: "duration"},
			cli.IntFlag{Name: "limit"},
			cli.Float64Flag{Name: "ratio"},
		},
		[]string{"--duration", "5s", "--limit", "7", "--ratio", "0.25"},
	)
	f := cliflags.NewV1(c)
	assert.Equal(t, 5*time.Second, f.Duration("duration"))
	assert.Equal(t, 7, f.Int("limit"))
	assert.InEpsilon(t, 0.25, f.Float64("ratio"), 1e-9)
}

func TestV1_StringSlice(t *testing.T) {
	c := newCtx(t,
		[]cli.Flag{cli.StringSliceFlag{Name: "tag"}},
		[]string{"--tag", "a", "--tag", "b"},
	)
	f := cliflags.NewV1(c)
	assert.Equal(t, []string{"a", "b"}, f.StringSlice("tag"))
}

func TestV1_Args(t *testing.T) {
	c := newCtx(t, nil, []string{"alpha", "beta", "gamma"})
	f := cliflags.NewV1(c)
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, f.Args())
}

func TestV1_ArgsEmpty(t *testing.T) {
	c := newCtx(t, nil, nil)
	f := cliflags.NewV1(c)
	assert.Empty(t, f.Args())
}

func TestV1_ParentNilAtRoot(t *testing.T) {
	c := newCtx(t, nil, nil)
	f := cliflags.NewV1(c)
	assert.Nil(t, f.Parent(), "root context has no parent")
}

func TestV1_ParentReadsParentFlag(t *testing.T) {
	parent := newCtx(t,
		[]cli.Flag{cli.StringFlag{Name: "interface"}},
		[]string{"--interface", "eth0"},
	)
	child := childCtx(t, parent,
		[]cli.Flag{cli.IntFlag{Name: "limit"}},
		[]string{"--limit", "3"},
	)
	f := cliflags.NewV1(child)
	require.NotNil(t, f.Parent())
	assert.Equal(t, "eth0", f.Parent().String("interface"))
	// And child still sees its own flag.
	assert.Equal(t, 3, f.Int("limit"))
}

func TestV1_GlobalWalksToRoot(t *testing.T) {
	root := newCtx(t,
		[]cli.Flag{cli.BoolFlag{Name: "dry-run"}},
		[]string{"--dry-run"},
	)
	mid := childCtx(t, root,
		[]cli.Flag{cli.StringFlag{Name: "interface"}},
		[]string{"--interface", "eth0"},
	)
	leaf := childCtx(t, mid,
		[]cli.Flag{cli.IntFlag{Name: "time"}},
		[]string{"--time", "100"},
	)
	f := cliflags.NewV1(leaf)
	g := f.Global()
	require.NotNil(t, g)
	assert.True(t, g.Bool("dry-run"), "Global() must walk up to the root")
	assert.Equal(t, 100, f.Int("time"))
}

func TestV1_GlobalAtRootReturnsRoot(t *testing.T) {
	c := newCtx(t,
		[]cli.Flag{cli.BoolFlag{Name: "x"}},
		[]string{"--x"},
	)
	f := cliflags.NewV1(c)
	assert.True(t, f.Global().Bool("x"))
}
