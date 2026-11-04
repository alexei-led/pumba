package tc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecoveryDeleteCommandGrammarBoundaries(t *testing.T) {
	valid := `-A FORWARD -s 10.0.0.1/32 -d 10.0.0.2/32 -i eth0 -o eth1 -p tcp --sport 10:20 --dport 80 -m tcp -m udp -m comment --comment pumba:run_1 -m statistic --mode nth --every 3 --packet 2 --probability 0.50000000000 -j DROP`
	got, err := RecoveryDeleteCommand("iptables-legacy", "run_1", "FORWARD", valid)
	require.NoError(t, err)
	require.Equal(t, []string{"iptables-legacy", "-t", "filter", "-D", "FORWARD", "-s", "10.0.0.1/32", "-d", "10.0.0.2/32", "-i", "eth0", "-o", "eth1", "-p", "tcp", "--sport", "10:20", "--dport", "80", "-m", "tcp", "-m", "udp", "-m", "comment", "--comment", "pumba:run_1", "-m", "statistic", "--mode", "nth", "--every", "3", "--packet", "2", "--probability", "0.50000000000", "-j", "DROP"}, got)

	for _, tc := range []struct {
		name, backend, runID, chain, spec, want string
	}{
		{name: "bad run id", backend: "iptables-nft", runID: "bad/id", chain: "INPUT", spec: `-A INPUT -m comment --comment pumba:bad/id -j DROP`, want: "invalid recovery run ID"},
		{name: "unsupported chain", backend: "iptables-nft", runID: "r", chain: "CUSTOM", spec: `-A CUSTOM -m comment --comment pumba:r -j DROP`, want: "invalid recovery rule header"},
		{name: "unsupported match", backend: "iptables-nft", runID: "r", chain: "INPUT", spec: `-A INPUT -m recent --name x -m comment --comment pumba:r -j DROP`, want: "unsupported recovery match"},
		{name: "unsupported statistic mode", backend: "iptables-nft", runID: "r", chain: "INPUT", spec: `-A INPUT -m statistic --mode randomx -m comment --comment pumba:r -j DROP`, want: "unsupported statistic mode"},
		{name: "comment module must precede owner comment", backend: "iptables-nft", runID: "r", chain: "INPUT", spec: `-A INPUT --comment pumba:r -m comment -j DROP`, want: "ambiguous recovery ownership"},
		{name: "option may occur once", backend: "iptables-nft", runID: "r", chain: "INPUT", spec: `-A INPUT -s 10.0.0.1 -s 10.0.0.2 -m comment --comment pumba:r -j DROP`, want: "duplicate recovery option"},
		{name: "value cannot be an option", backend: "iptables-nft", runID: "r", chain: "INPUT", spec: `-A INPUT -s -d -m comment --comment pumba:r -j DROP`, want: "invalid recovery value"},
		{name: "ownership module cannot repeat", backend: "iptables-nft", runID: "r", chain: "INPUT", spec: `-A INPUT -m comment -m comment --comment pumba:r -j DROP`, want: "multiple ownership modules"},
		{name: "owner comment is required", backend: "iptables-nft", runID: "r", chain: "INPUT", spec: `-A INPUT -j DROP`, want: "requires one ownership comment"},
		{name: "only drop target is accepted", backend: "iptables-nft", runID: "r", chain: "INPUT", spec: `-A INPUT -m comment --comment pumba:r -j ACCEPT`, want: "unsupported recovery target"},
		{name: "unknown option rejected", backend: "iptables-nft", runID: "r", chain: "INPUT", spec: `-A INPUT --random x -m comment --comment pumba:r -j DROP`, want: "unsupported recovery option"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RecoveryDeleteCommand(tc.backend, tc.runID, tc.chain, tc.spec)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
