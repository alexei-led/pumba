package netns

import (
	"context"
	"net"
	"slices"
	"strings"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/require"
)

// Exercise the combined compiler -> inventory -> recovery boundary, including
// duplicate occurrences and a second target sharing the same namespace.
func TestRecoverMatchAllRules(t *testing.T) {
	_, source, err := net.ParseCIDR("192.0.2.1/32")
	require.NoError(t, err)
	_, destination, err := net.ParseCIDR("198.51.100.2/32")
	require.NoError(t, err)
	commands, err := IPTablesCommands(&ctr.IPTablesRequest{
		CmdPrefix: []string{"-A", "INPUT", "-p", "tcp", "-m", "tcp"},
		CmdSuffix: []string{"-m", "statistic", "--mode", "random", "--probability", "0.50000000000", "-j", "DROP"},
		SrcIPs:    []*net.IPNet{source}, DstIPs: []*net.IPNet{destination},
		SPorts: []string{"12345"}, DPorts: []string{"443", "8443"}, MatchAll: true, RunID: "old",
	})
	require.NoError(t, err)
	require.Len(t, commands, 2)

	present := map[string][]string{}
	var expected [][]string
	const foreign = "-A INPUT -m comment --comment pumba:other -j DROP"
	for _, backend := range []string{"iptables-legacy", "iptables-nft"} {
		// Repeat one occurrence without broadening or deduplicating the plan.
		for _, argv := range append(slices.Clone(commands), commands[0]) {
			present[backend] = append(present[backend], strings.Join(argv[1:], " "))
			expected = append(expected, append([]string{backend, "-t", "filter", "-D", "INPUT"}, argv[3:]...))
		}
		present[backend] = append(present[backend], foreign)
	}
	var deleted [][]string
	run := func(argv []string) (string, string, error) {
		switch {
		case slices.Equal(argv, []string{"tc", "qdisc", "show"}):
			return "", "", nil
		case slices.Equal(argv, []string{"iptables", "--version"}):
			return "iptables v1.8.13 (legacy)", "", nil
		case len(argv) == 2 && argv[1] == "-S":
			return strings.Join(present[argv[0]], "\n"), "", nil
		default:
			require.GreaterOrEqual(t, len(argv), 5)
			require.Equal(t, []string{"-t", "filter", "-D", "INPUT"}, argv[1:5])
			spec := "-A INPUT " + strings.Join(argv[5:], " ")
			index := slices.Index(present[argv[0]], spec)
			require.NotEqual(t, -1, index, "delete must preserve the entire AND rule")
			present[argv[0]] = slices.Delete(present[argv[0]], index, index+1)
			deleted = append(deleted, slices.Clone(argv))
			return "", "", nil
		}
	}
	state, err := Inspect(run)
	require.NoError(t, err)
	req := &ctr.RuleRecoveryRequest{Container: &ctr.Container{ContainerID: "first"}, RunFilter: "old"}
	for _, rule := range state.Rules {
		if rule.RunID == req.RunFilter {
			req.Rules = append(req.Rules, rule)
		}
	}
	plan, err := RecoveryPlan(req)
	require.NoError(t, err)
	require.Equal(t, expected, plan.Revert)
	for _, id := range []string{"first", "shared"} {
		req.Container = &ctr.Container{ContainerID: id}
		require.NoError(t, RecoverRules(t.Context(), req, func(_ context.Context, argv []string) (string, string, error) {
			return run(argv)
		}))
	}
	require.Equal(t, expected, deleted, "second target must accept verified absence without another delete")
	for _, remaining := range present {
		require.Equal(t, []string{foreign}, remaining)
	}
}
