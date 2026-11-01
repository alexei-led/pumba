package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

// noRuntime makes every runtime factory fail the test if it is called.
func noRuntime(t *testing.T) {
	t.Helper()
	restoreFactories(t)
	newDockerClient = func(string, *tls.Config) (ctr.Client, error) {
		t.Error("schema created a Docker client")
		return nil, assert.AnError
	}
	newContainerdClient = func(string, string) (ctr.Client, error) {
		t.Error("schema created a containerd client")
		return nil, assert.AnError
	}
	newPodmanClient = func(string) (ctr.Client, error) {
		t.Error("schema created a podman client")
		return nil, assert.AnError
	}
}

// runSchema runs `pumba <args...>` without a runtime and returns stdout.
func runSchema(t *testing.T, args ...string) string {
	t.Helper()
	noRuntime(t)
	var stdout bytes.Buffer
	code := run(append([]string{"pumba"}, args...), &stdout, io.Discard)
	require.Zero(t, code)
	return stdout.String()
}

func decodeSchema(t *testing.T, out string) cliSchemaDoc {
	t.Helper()
	var doc cliSchemaDoc
	require.NoError(t, json.Unmarshal([]byte(out), &doc), "stdout must be exactly one JSON document")
	return doc
}

func commandPaths(cmds []cliflags.CommandInfo) []string {
	var out []string
	for _, c := range cmds {
		out = append(out, c.Path)
		out = append(out, commandPaths(c.Subcommands)...)
	}
	return out
}

func declaredPaths(cmds []cli.Command, parent string) []string {
	var out []string
	for _, c := range cmds {
		path := c.Name
		if parent != "" {
			path = parent + " " + c.Name
		}
		out = append(out, path)
		out = append(out, declaredPaths(c.Subcommands, path)...)
	}
	return out
}

func findCommand(cmds []cliflags.CommandInfo, path string) *cliflags.CommandInfo {
	for i := range cmds {
		if cmds[i].Path == path {
			return &cmds[i]
		}
		if c := findCommand(cmds[i].Subcommands, path); c != nil {
			return c
		}
	}
	return nil
}

func TestSchemaRunsWithoutRuntime(t *testing.T) {
	for _, args := range [][]string{
		{"schema"},
		{"-o", "ndjson", "schema"},
		{"--runtime", "containerd", "schema"},
		{"--runtime", "podman", "--json", "schema"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			doc := decodeSchema(t, runSchema(t, args...))
			assert.Equal(t, "pumba.cli/v1", doc.Schema)
			assert.Equal(t, version, doc.PumbaVersion)
			assert.Equal(t, chaos.EventSchema, doc.EventSchema)
			assert.Equal(t, chaos.ErrorCodes(), doc.ErrorCodes)
			assert.Equal(t, chaos.RunStatuses(), doc.RunStatuses)
			assert.Equal(t, chaos.TargetStatuses(), doc.TargetStatuses)
		})
	}
}

func TestSchemaTopLevelKeys(t *testing.T) {
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(runSchema(t, "schema")), &raw))
	assert.ElementsMatch(t, []string{
		"schema", "pumba_version", "event_schema", "error_codes", "run_statuses", "target_statuses",
		"usage", "args_usage", "global_flags", "commands",
	}, slices.Collect(maps.Keys(raw)))
}

func TestSchemaCoversEveryCommand(t *testing.T) {
	doc := decodeSchema(t, runSchema(t, "schema"))
	want := append(declaredPaths(initializeCLICommands(nil, nil), ""), schemaCommandName)
	got := commandPaths(doc.Commands)

	assert.Equal(t, want, got, "every command in declaration order, help omitted")
	assert.Subset(t, got, []string{"kill", "stress", "netem", "netem delay", "netem loss-gemodel", "iptables loss", "schema"})
	assert.NotContains(t, got, "help")
}

func TestSchemaKnownFlags(t *testing.T) {
	doc := decodeSchema(t, runSchema(t, "schema"))
	tests := []struct {
		path string // "" for global flags
		want cliflags.FlagInfo
	}{
		{"", cliflags.FlagInfo{Name: "host", Aliases: []string{"H"}, Type: "string", Default: "unix:///var/run/docker.sock", EnvVars: []string{"DOCKER_HOST"}, Usage: "daemon socket to connect to"}},
		{"", cliflags.FlagInfo{Name: "max-targets", Aliases: []string{}, Type: "int", Default: float64(0), EnvVars: []string{}, Usage: "refuse chaos when selected targets exceed this cap after --limit/--random (0: unset)"}},
		{"", cliflags.FlagInfo{Name: "label", Aliases: []string{}, Type: "string[]", Default: []any{}, EnvVars: []string{}, Repeatable: true, Usage: "filter containers by labels, e.g. '--label key=value' (use '--label k1=v1 --label k2=v2' or '--label k1=v1,k2=v2' for multiple, AND logic)"}},
		{"", cliflags.FlagInfo{Name: "dry-run", Aliases: []string{}, Type: "bool", Default: false, EnvVars: []string{"DRY-RUN"}, Usage: "dry run does not create chaos, only logs planned chaos commands"}},
		{"", cliflags.FlagInfo{Name: "output", Aliases: []string{"o"}, Type: "string", Default: "text", EnvVars: []string{}, Usage: "result output: text (logs only) or ndjson (pumba.event/v1 events on stdout, one per line)"}},
		{"netem", cliflags.FlagInfo{Name: "duration", Aliases: []string{"d"}, Type: "duration", Default: "0s", EnvVars: []string{}, Usage: "network emulation duration; should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h'"}},
		{"netem", cliflags.FlagInfo{Name: "pull-image", Aliases: []string{}, Type: "bool", Default: true, EnvVars: []string{}, Usage: "force pull tc-image"}},
		{"netem delay", cliflags.FlagInfo{Name: "time", Aliases: []string{"t"}, Type: "int", Default: float64(100), EnvVars: []string{}, Usage: "delay time; in milliseconds"}},
		{"netem loss", cliflags.FlagInfo{Name: "percent", Aliases: []string{"p"}, Type: "number", Default: float64(0), EnvVars: []string{}, Usage: "packet loss percentage"}},
		{"iptables", cliflags.FlagInfo{Name: "source", Aliases: []string{"src", "s"}, Type: "string[]", Default: []any{}, EnvVars: []string{}, Repeatable: true, Usage: "source IP filter; supports multiple IPs; supports CIDR notation"}},
	}
	for _, tt := range tests {
		t.Run(tt.path+" --"+tt.want.Name, func(t *testing.T) {
			flags := doc.GlobalFlags
			if tt.path != "" {
				c := findCommand(doc.Commands, tt.path)
				require.NotNil(t, c, "command %q", tt.path)
				flags = c.Flags
			}
			i := slices.IndexFunc(flags, func(f cliflags.FlagInfo) bool { return f.Name == tt.want.Name })
			require.GreaterOrEqual(t, i, 0, "flag --%s", tt.want.Name)
			assert.Equal(t, tt.want, flags[i])
		})
	}
}

func TestSchemaOmitsHelpAndHiddenFlags(t *testing.T) {
	doc := decodeSchema(t, runSchema(t, "schema"))
	var names []string
	for _, f := range doc.GlobalFlags {
		names = append(names, f.Name)
	}
	assert.NotContains(t, names, "help")
	assert.NotContains(t, names, "generate-bash-completion", "hidden flag")
	assert.Contains(t, names, "version")
}

func TestSchemaIsDeterministic(t *testing.T) {
	first := runSchema(t, "schema")
	assert.Equal(t, first, runSchema(t, "schema"))
}

func TestSchemaFailsOnUnknownFlagType(t *testing.T) {
	tests := []struct {
		name string
		app  *cli.App
	}{
		{"global flag", &cli.App{Flags: []cli.Flag{cli.UintFlag{Name: "workers"}}}},
		{"command flag", &cli.App{Commands: []cli.Command{{Name: "stress", Flags: []cli.Flag{cli.Int64Flag{Name: "workers"}}}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := writeSchema(&out, tt.app)
			require.ErrorIs(t, err, cliflags.ErrUnsupportedFlagType)
			assert.Empty(t, out.String(), "no partial document")
		})
	}
}
