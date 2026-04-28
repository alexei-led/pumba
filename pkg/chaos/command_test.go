package chaos

import (
	"context"
	"errors"
	"flag"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

// buildFlags constructs a cliflags.Flags from a root cli.Context carrying the
// given global flags + args, with an optional child context (for subcommand
// arg scenarios where args live on the child).
func buildFlags(t *testing.T, globalFlags []cli.Flag, globalArgs []string, childFlags []cli.Flag, childArgs []string) cliflags.Flags {
	t.Helper()
	app := cli.NewApp()
	rootFS := flag.NewFlagSet("root", flag.ContinueOnError)
	for _, f := range globalFlags {
		f.Apply(rootFS)
	}
	require.NoError(t, rootFS.Parse(globalArgs))
	rootCtx := cli.NewContext(app, rootFS, nil)

	if childFlags == nil && childArgs == nil {
		return cliflags.NewV1(rootCtx)
	}
	childFS := flag.NewFlagSet("sub", flag.ContinueOnError)
	for _, f := range childFlags {
		f.Apply(childFS)
	}
	require.NoError(t, childFS.Parse(childArgs))
	childCtx := cli.NewContext(app, childFS, rootCtx)
	return cliflags.NewV1(childCtx)
}

func TestSplitLabels(t *testing.T) {
	tests := []struct {
		name     string
		raw      []string
		expected []string
	}{
		{"nil input", nil, nil},
		{"empty input", []string{}, nil},
		{"single label", []string{"k=v"}, []string{"k=v"}},
		{"multiple flags", []string{"k1=v1", "k2=v2"}, []string{"k1=v1", "k2=v2"}},
		{"comma separated", []string{"k1=v1,k2=v2"}, []string{"k1=v1", "k2=v2"}},
		{"mixed", []string{"k1=v1,k2=v2", "k3=v3"}, []string{"k1=v1", "k2=v2", "k3=v3"}},
		{"whitespace trimmed", []string{" k1=v1 , k2=v2 "}, []string{"k1=v1", "k2=v2"}},
		{"empty parts skipped", []string{"k1=v1,,k2=v2"}, []string{"k1=v1", "k2=v2"}},
		{"only commas", []string{",,"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitLabels(tt.raw)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// mockCommand implements Command for testing
type mockCommand struct {
	err   error
	calls int
}

func (m *mockCommand) Run(_ context.Context, _ bool) error {
	m.calls++
	return m.err
}

func TestRunChaosCommand_SingleRun(t *testing.T) {
	cmd := &mockCommand{}
	params := &GlobalParams{Interval: 0}

	err := RunChaosCommand(context.Background(), cmd, params)
	assert.NoError(t, err)
	assert.Equal(t, 1, cmd.calls)
}

func TestRunChaosCommand_Error(t *testing.T) {
	cmd := &mockCommand{err: errors.New("chaos failed")}
	params := &GlobalParams{Interval: 0}

	err := RunChaosCommand(context.Background(), cmd, params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chaos failed")
}

func TestRunChaosCommand_SkipErrors(t *testing.T) {
	cmd := &mockCommand{err: errors.New("chaos failed")}
	params := &GlobalParams{Interval: 0, SkipErrors: true}

	err := RunChaosCommand(context.Background(), cmd, params)
	assert.NoError(t, err)
}

func TestRunChaosCommand_ContextCancel(t *testing.T) {
	cmd := &mockCommand{}
	params := &GlobalParams{Interval: time.Hour} // long interval

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := RunChaosCommand(ctx, cmd, params)
	assert.NoError(t, err)
	assert.Equal(t, 1, cmd.calls)
}

func TestRuntime_ReturnsInjectedClient(t *testing.T) {
	want := container.NewMockClient(t)

	var runtime Runtime = func() container.Client { return want }

	assert.Same(t, want, runtime(), "Runtime factory must return the injected client")
}

func TestRuntime_DefersClientResolution(t *testing.T) {
	var holder container.Client

	runtime := Runtime(func() container.Client { return holder })

	assert.Nil(t, runtime(), "Runtime should resolve client lazily — nil before assignment")

	first := container.NewMockClient(t)
	holder = first
	assert.Same(t, first, runtime(), "Runtime should observe later client assignment")

	second := container.NewMockClient(t)
	holder = second
	assert.Same(t, second, runtime(), "Runtime should re-read holder on every call")
}

func TestGetNamesOrPattern(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantNames   []string
		wantPattern string
	}{
		{
			name:        "no args — target all containers",
			args:        nil,
			wantNames:   nil,
			wantPattern: "",
		},
		{
			name:        "single plain name",
			args:        []string{"web"},
			wantNames:   []string{"web"},
			wantPattern: "",
		},
		{
			name:        "multiple names",
			args:        []string{"web", "db", "cache"},
			wantNames:   []string{"web", "db", "cache"},
			wantPattern: "",
		},
		{
			name:        "re2 prefix yields pattern",
			args:        []string{"re2:^app-"},
			wantNames:   nil,
			wantPattern: "^app-",
		},
		{
			name:        "re2 prefix with empty pattern",
			args:        []string{"re2:"},
			wantNames:   nil,
			wantPattern: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := buildFlags(t, nil, nil, nil, tt.args)
			names, pattern := getNamesOrPattern(f)
			assert.Equal(t, tt.wantNames, names)
			assert.Equal(t, tt.wantPattern, pattern)
		})
	}
}

func TestParseGlobalParams(t *testing.T) {
	globalFlags := []cli.Flag{
		cli.BoolFlag{Name: "random"},
		cli.BoolFlag{Name: "dry-run"},
		cli.BoolFlag{Name: "skip-error"},
		cli.DurationFlag{Name: "interval"},
		cli.StringSliceFlag{Name: "label"},
	}

	tests := []struct {
		name       string
		globalArgs []string
		childArgs  []string
		want       *GlobalParams
	}{
		{
			name:       "defaults with no args",
			globalArgs: nil,
			childArgs:  nil,
			want: &GlobalParams{
				Random:     false,
				DryRun:     false,
				SkipErrors: false,
				Interval:   0,
				Labels:     nil,
				Names:      nil,
				Pattern:    "",
			},
		},
		{
			name:       "random and dry-run flags",
			globalArgs: []string{"--random", "--dry-run"},
			childArgs:  nil,
			want: &GlobalParams{
				Random: true,
				DryRun: true,
			},
		},
		{
			name:       "skip-error and interval",
			globalArgs: []string{"--skip-error", "--interval", "30s"},
			childArgs:  nil,
			want: &GlobalParams{
				SkipErrors: true,
				Interval:   30 * time.Second,
			},
		},
		{
			name:       "labels comma-separated",
			globalArgs: []string{"--label", "env=prod,tier=web"},
			childArgs:  nil,
			want: &GlobalParams{
				Labels: []string{"env=prod", "tier=web"},
			},
		},
		{
			name:       "labels multiple flags",
			globalArgs: []string{"--label", "env=prod", "--label", "tier=web"},
			childArgs:  nil,
			want: &GlobalParams{
				Labels: []string{"env=prod", "tier=web"},
			},
		},
		{
			name:       "names from child args",
			globalArgs: nil,
			childArgs:  []string{"web", "db"},
			want: &GlobalParams{
				Names: []string{"web", "db"},
			},
		},
		{
			name:       "re2 pattern from child args",
			globalArgs: nil,
			childArgs:  []string{"re2:^app-"},
			want: &GlobalParams{
				Pattern: "^app-",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := buildFlags(t, globalFlags, tt.globalArgs, []cli.Flag{}, tt.childArgs)
			got := ParseGlobalParams(f)
			// Selectively assert only the fields the test specifies.
			if tt.want.Random {
				assert.True(t, got.Random)
			}
			if tt.want.DryRun {
				assert.True(t, got.DryRun)
			}
			if tt.want.SkipErrors {
				assert.True(t, got.SkipErrors)
			}
			if tt.want.Interval != 0 {
				assert.Equal(t, tt.want.Interval, got.Interval)
			}
			if tt.want.Labels != nil {
				assert.Equal(t, tt.want.Labels, got.Labels)
			}
			if tt.want.Names != nil {
				assert.Equal(t, tt.want.Names, got.Names)
			}
			if tt.want.Pattern != "" {
				assert.Equal(t, tt.want.Pattern, got.Pattern)
			}
		})
	}
}
