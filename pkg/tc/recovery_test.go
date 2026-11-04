package tc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecoveryDeleteCommand(t *testing.T) {
	spec := `-A INPUT -s 10.0.0.1/32 -p tcp -m tcp --dport 443 -m comment --comment "pumba:old" -m statistic --mode random --probability 0.50000000000 -j DROP`
	got, err := RecoveryDeleteCommand("iptables-nft", "old", "INPUT", spec)
	require.NoError(t, err)
	require.Equal(t, []string{"iptables-nft", "-t", "filter", "-D", "INPUT", "-s", "10.0.0.1/32", "-p", "tcp", "-m", "tcp", "--dport", "443", "-m", "comment", "--comment", "pumba:old", "-m", "statistic", "--mode", "random", "--probability", "0.50000000000", "-j", "DROP"}, got)
	for _, bad := range []string{
		`-A INPUT -m comment --comment pumba:other -j DROP`,
		`-A INPUT -m comment --comment pumba:old --comment pumba:old -j DROP`,
		`-A INPUT -m comment --comment pumba:old -t nat -j DROP`,
		`-A INPUT -m comment --comment pumba:old -F`,
		`-A INPUT -m comment --comment pumba:old -j DROP; touch /tmp/x`,
		`-A OUTPUT -m comment --comment pumba:old -j DROP`,
		`-D INPUT -m comment --comment pumba:old -j DROP`,
		`-A INPUT --comment pumba:old -j DROP`,
		`-A INPUT -m comment --comment "pumba:old`,
	} {
		t.Run(bad, func(t *testing.T) {
			_, err := RecoveryDeleteCommand("iptables-legacy", "old", "INPUT", bad)
			require.Error(t, err)
		})
	}
	_, err = RecoveryDeleteCommand("iptables", "old", "INPUT", spec)
	require.Error(t, err)
}

func TestInspectRejectsMultipleOwnershipComments(t *testing.T) {
	for _, spec := range []string{
		`-A INPUT -m comment --comment pumba:other -m comment --comment pumba:old -j DROP`,
		`-A INPUT -m comment --comment pumba:old -m comment --comment other -j DROP`,
	} {
		_, err := ParseTaggedRules(spec)
		require.ErrorContains(t, err, "ambiguous ownership")
	}
}
