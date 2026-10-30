package tc

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The untagged cases pin the argv the docker and containerd runtimes built
// before the builder moved here; they must stay byte-for-byte identical.
func TestIPTablesCommands(t *testing.T) {
	prefix := []string{"-I", "INPUT", "-i", "eth0", "-p", "tcp"}
	suffix := []string{"-m", "statistic", "--mode", "random", "--probability", "0.20", "-j", "DROP"}

	tests := []struct {
		name string
		req  IPTablesRequest
		want [][]string
	}{
		{
			name: "no filters",
			req:  IPTablesRequest{CmdPrefix: []string{"-A", "INPUT"}, CmdSuffix: []string{"-j", "DROP"}},
			want: [][]string{{"-A", "INPUT", "-j", "DROP"}},
		},
		{
			name: "every filter kind in order",
			req: IPTablesRequest{
				CmdPrefix: prefix, CmdSuffix: suffix,
				SrcIPs: []string{"10.0.0.1/32", "10.1.0.0/16"}, DstIPs: []string{"192.168.1.0/24"},
				SPorts: []string{"80"}, DPorts: []string{"53", "443"},
			},
			want: [][]string{
				{"-I", "INPUT", "-i", "eth0", "-p", "tcp", "-s", "10.0.0.1/32", "-m", "statistic", "--mode", "random", "--probability", "0.20", "-j", "DROP"},
				{"-I", "INPUT", "-i", "eth0", "-p", "tcp", "-s", "10.1.0.0/16", "-m", "statistic", "--mode", "random", "--probability", "0.20", "-j", "DROP"},
				{"-I", "INPUT", "-i", "eth0", "-p", "tcp", "-d", "192.168.1.0/24", "-m", "statistic", "--mode", "random", "--probability", "0.20", "-j", "DROP"},
				{"-I", "INPUT", "-i", "eth0", "-p", "tcp", "--sport", "80", "-m", "statistic", "--mode", "random", "--probability", "0.20", "-j", "DROP"},
				{"-I", "INPUT", "-i", "eth0", "-p", "tcp", "--dport", "53", "-m", "statistic", "--mode", "random", "--probability", "0.20", "-j", "DROP"},
				{"-I", "INPUT", "-i", "eth0", "-p", "tcp", "--dport", "443", "-m", "statistic", "--mode", "random", "--probability", "0.20", "-j", "DROP"},
			},
		},
		{
			name: "tagged without filters",
			req:  IPTablesRequest{CmdPrefix: []string{"-I", "INPUT"}, CmdSuffix: []string{"-j", "DROP"}, RunID: "run1"},
			want: [][]string{{"-I", "INPUT", "-m", "comment", "--comment", "pumba:run1", "-j", "DROP"}},
		},
		{
			name: "tagged comment sits between filter and suffix",
			req:  IPTablesRequest{CmdPrefix: []string{"-D", "INPUT"}, CmdSuffix: []string{"-j", "DROP"}, SPorts: []string{"22"}, RunID: "a.b-c"},
			want: [][]string{{"-D", "INPUT", "--sport", "22", "-m", "comment", "--comment", "pumba:a.b-c", "-j", "DROP"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IPTablesCommands(&tt.req))
		})
	}
}

func TestIPTablesCommandsAddAndDeleteDifferOnlyInOperation(t *testing.T) {
	add := IPTablesRequest{CmdPrefix: []string{"-I", "INPUT"}, CmdSuffix: []string{"-j", "DROP"}, DstIPs: []string{"10.0.0.0/8"}, RunID: "r"}
	del := add
	del.CmdPrefix = []string{"-D", "INPUT"}

	addCmds, delCmds := IPTablesCommands(&add), IPTablesCommands(&del)

	require.Len(t, delCmds, len(addCmds))
	for i := range addCmds {
		assert.Equal(t, addCmds[i][1:], delCmds[i][1:])
	}
}

func TestExplainIPTablesError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		tagged      bool
		wantComment bool
	}{
		{name: "nil", err: nil, tagged: true},
		{name: "legacy missing extension", err: errors.New("iptables v1.8.9 (legacy): Couldn't load match `comment':No such file or directory"), tagged: true, wantComment: true},
		{name: "nft missing kernel module", err: errors.New("Warning: Extension comment revision 0 not supported, missing kernel module?\niptables: No chain/target/match by that name."), tagged: true, wantComment: true},
		{name: "generic no match", err: errors.New("exit code 1: iptables: No chain/target/match by that name."), tagged: true, wantComment: true},
		{name: "untagged rule", err: errors.New("Couldn't load match `comment'"), tagged: false},
		{name: "unrelated failure", err: errors.New("iptables: Permission denied (you must be root)"), tagged: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExplainIPTablesError(tt.err, tt.tagged)
			if tt.err == nil {
				require.NoError(t, got)
				return
			}
			require.ErrorIs(t, got, tt.err)
			assert.Equal(t, tt.wantComment, errors.Is(got, ErrCommentMatchUnavailable))
			if tt.wantComment {
				assert.Contains(t, got.Error(), "xt_comment")
			}
		})
	}
}
