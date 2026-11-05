package tc

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchAllStopVerifiesGroupedFilters(t *testing.T) {
	cases := []struct {
		name, filters string
		fail          bool
	}{
		{name: "merged keys", filters: "match 0a000001/ffffffff at 16\n  match 005001bb/ffffffff at 20"},
		{name: "split keys", filters: "match 0a000001/ffffffff at 16\n  match 00500000/ffff0000 at 20\n  match 000001bb/0000ffff at 20"},
		{name: "reordered keys", filters: "match 000001bb/0000ffff at 20\n  match 0a000001/ffffffff at 16\n  match 00500000/ffff0000 at 20"},
		{name: "changed address", filters: "match 0a000002/ffffffff at 16\n  match 005001bb/ffffffff at 20", fail: true},
		{name: "changed port", filters: "match 0a000001/ffffffff at 16\n  match 005001bc/ffffffff at 20", fail: true},
		{name: "missing port", filters: "match 0a000001/ffffffff at 16\n  match 00500000/ffff0000 at 20", fail: true},
		{name: "duplicate key", filters: "match 0a000001/ffffffff at 16\n  match 005001bb/ffffffff at 20\n  match 005001bb/ffffffff at 20", fail: true},
		{name: "foreign hash table", filters: "match 0a000001/ffffffff at 16\n  match 005001bb/ffffffff at 20\nfilter protocol ip pref 1 u32 chain 0 fh 900: ht divisor 1", fail: true},
		{name: "foreign action", filters: "match 0a000001/ffffffff at 16\n  match 005001bb/ffffffff at 20\n  action drop", fail: true},
		{name: "duplicate filter", filters: "match 0a000001/ffffffff at 16\n  match 005001bb/ffffffff at 20\nfilter protocol ip pref 1 u32 chain 0 fh 800::801 flowid 504d:3\n  match 0a000001/ffffffff at 16\n  match 005001bb/ffffffff at 20", fail: true},
		{name: "foreign flow", filters: "match 0a000001/ffffffff at 16\n  match 005001bb/ffffffff at 20\nfilter protocol ip pref 1 u32 chain 0 fh 800::801 flowid 504d:2\n  match 0a000002/ffffffff at 16", fail: true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			args := stopArgs(t, &NetemRequest{Interface: "eth0", IPs: []string{"10.0.0.1/32"}, SPorts: []string{"80"}, DPorts: []string{"443"}, MatchAll: true})
			output, err := runScriptWithFailureAndFilter(t, args, scopedState(), "", tt.filters)
			if tt.fail {
				require.Error(t, err)
				assert.Empty(t, output)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "qdisc del dev eth0 root handle 504d:\n", output)
		})
	}
}

func TestMatchAllSelectorNormalizationAndIntersection(t *testing.T) {
	commands, err := IPTablesCommands(&IPTablesRequest{
		CmdPrefix: []string{"-I", "INPUT"}, CmdSuffix: []string{"-j", "DROP"}, RunID: "r",
		SrcIPs: []string{"10.1.2.3/8", "10.0.0.0/8", "10.1.0.0/16", "192.0.2.1/32"},
		SPorts: []string{"080", "80"}, DPorts: []string{"443", "8443"}, MatchAll: true,
	})
	require.NoError(t, err)
	require.Len(t, commands, 4)
	assert.Equal(t, []string{"-I", "INPUT", "-s", "10.0.0.0/8", "--sport", "80", "--dport", "443", "-m", "comment", "--comment", "pumba:r", "-j", "DROP"}, commands[0])
	for _, command := range commands {
		require.Len(t, command, 14)
		assert.Equal(t, []string{"-s", "--sport", "--dport"}, []string{command[2], command[4], command[6]})
	}
}

func TestMatchAllBoundsAndValidation(t *testing.T) {
	ports := make([]string, MaxFilterCombinations+1)
	for i := range ports {
		ports[i] = strconv.Itoa(i)
	}
	cases := []struct {
		name    string
		req     *NetemRequest
		limited bool
	}{
		{name: "boundary", req: &NetemRequest{Interface: "eth0", SPorts: ports[:MaxFilterCombinations], MatchAll: true}},
		{name: "one too many", req: &NetemRequest{Interface: "eth0", SPorts: ports, MatchAll: true}, limited: true},
		{name: "product too large", req: &NetemRequest{Interface: "eth0", SPorts: ports[:16], DPorts: ports[:17], MatchAll: true}, limited: true},
		{name: "script too large", req: &NetemRequest{Interface: "eth0", Command: []string{strings.Repeat("x", maxNetemScriptBytes)}, MatchAll: true}, limited: true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			start, err := Start(tt.req)
			stop, stopErr := Stop(tt.req)
			if tt.limited {
				require.ErrorIs(t, err, ErrFilterLimit)
				require.ErrorIs(t, stopErr, ErrFilterLimit)
				assert.Nil(t, start)
				assert.Nil(t, stop)
				return
			}
			require.NoError(t, err)
			require.NoError(t, stopErr)
			assert.LessOrEqual(t, len(start[1]), maxNetemScriptBytes)
			assert.LessOrEqual(t, len(stop[1]), maxNetemScriptBytes)
		})
	}
	commands, err := IPTablesCommands(&IPTablesRequest{SPorts: ports, MatchAll: true})
	require.ErrorIs(t, err, ErrFilterLimit)
	assert.Nil(t, commands)
	for _, req := range []*NetemRequest{
		{IPs: []string{"::/0"}, MatchAll: true},
		{IPs: []string{"invalid"}, MatchAll: true},
		{SPorts: []string{""}, MatchAll: true},
		{SPorts: []string{"65536"}, MatchAll: true},
	} {
		_, err := Start(req)
		require.Error(t, err, fmt.Sprintf("%+v", req))
	}
}

func TestMatchAllStartAndRollback(t *testing.T) {
	req := &NetemRequest{Interface: "eth0", Command: []string{"delay", "100ms"}, IPs: []string{"10.0.0.1/32"}, SPorts: []string{"80"}, DPorts: []string{"443"}, MatchAll: true}
	args := startArgs(t, req)
	log, err := runScript(t, args, "qdisc noqueue 0: root refcnt 2")
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(log, "filter add"))
	assert.Contains(t, log, "match ip dst 10.0.0.1/32 match ip sport 80 0xffff match ip dport 443 0xffff")
	log, err = runScriptWithFailure(t, args, "qdisc noqueue 0: root refcnt 2", "filter add")
	require.Error(t, err)
	assert.Contains(t, log, "qdisc del dev eth0 root handle 504d:")
}

func TestMatchAllWithoutSelectorsKeepsUnscopedBehavior(t *testing.T) {
	req := &NetemRequest{Interface: "eth0", Command: []string{"delay", "100ms"}}
	plainStart := startArgs(t, req)
	plainStop := stopArgs(t, req)
	req.MatchAll = true
	assert.Equal(t, plainStart, startArgs(t, req))
	assert.Equal(t, plainStop, stopArgs(t, req))
}

func scopedState() string {
	return strings.Join([]string{
		"qdisc prio 504d: root refcnt 2 bands 3 priomap",
		"qdisc sfq 504e: parent 504d:1 limit 127p",
		"qdisc sfq 504f: parent 504d:2 limit 127p",
		"qdisc netem 5050: parent 504d:3 limit 1000",
	}, "\n")
}
