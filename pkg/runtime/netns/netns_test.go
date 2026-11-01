package netns

import (
	"errors"
	"net"
	"strings"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeRunner(outputs, stderrs map[string]string, failures map[string]error) Runner {
	return func(argv []string) (string, string, error) {
		key := strings.Join(argv, " ")
		if err, ok := failures[key]; ok {
			return "", "", err
		}
		return outputs[key], stderrs[key], nil
	}
}

const legacyWarning = "# Warning: iptables-legacy tables present, use iptables-legacy to see them\n"

func TestInspect(t *testing.T) {
	scoped := "qdisc prio 504d: dev eth0 root refcnt 2 bands 3\n" +
		"qdisc sfq 504e: dev eth0 parent 504d:1 limit 127p\n" +
		"qdisc sfq 504f: dev eth0 parent 504d:2 limit 127p\n" +
		"qdisc netem 5050: dev eth0 parent 504d:3 limit 1000 delay 1s\n" +
		"qdisc tbf 504d: dev eth1 root refcnt 2 rate 1Mbit\n"
	rules := "-P INPUT ACCEPT\n-A INPUT -m comment --comment \"pumba:a\" -j DROP\n-A OUTPUT -j ACCEPT\n"

	tests := []struct {
		name     string
		outputs  map[string]string
		stderrs  map[string]string
		failures map[string]error
		want     *ctr.FaultState
		wantErr  string
	}{
		{
			name:    "clean netns",
			outputs: map[string]string{"tc qdisc show": "qdisc noqueue 0: dev eth0 root refcnt 2\n", "iptables -S": "-P INPUT ACCEPT\n"},
			want:    &ctr.FaultState{},
		},
		{
			name:    "qdiscs and tagged rules",
			outputs: map[string]string{"tc qdisc show": scoped, "iptables -S": rules},
			want: &ctr.FaultState{
				Qdiscs: []ctr.PumbaQdisc{
					{Device: "eth0", Kind: "prio", Ownership: ctr.QdiscPumbaOwned},
					{Device: "eth1", Kind: "tbf", Ownership: ctr.QdiscSuspicious},
				},
				Rules: []ctr.PumbaRule{{RunID: "a", Chain: "INPUT", Spec: "-A INPUT -m comment --comment \"pumba:a\" -j DROP"}},
			},
		},
		{
			name:    "legacy warning on stderr lists legacy rules too",
			outputs: map[string]string{"tc qdisc show": "qdisc noqueue 0: dev eth0 root refcnt 2\n", "iptables -S": rules, "iptables-legacy -S": "-A FORWARD -m comment --comment \"pumba:b\" -j DROP\n"},
			stderrs: map[string]string{"iptables -S": legacyWarning},
			want: &ctr.FaultState{Rules: []ctr.PumbaRule{
				{RunID: "a", Chain: "INPUT", Spec: "-A INPUT -m comment --comment \"pumba:a\" -j DROP"},
				{RunID: "b", Chain: "FORWARD", Spec: "-A FORWARD -m comment --comment \"pumba:b\" -j DROP"},
			}},
		},
		{
			name:    "legacy warning on stdout lists legacy rules too",
			outputs: map[string]string{"tc qdisc show": "qdisc noqueue 0: dev eth0 root refcnt 2\n", "iptables -S": "-P INPUT ACCEPT\n" + legacyWarning, "iptables-legacy -S": "-A FORWARD -m comment --comment \"pumba:b\" -j DROP\n"},
			want:    &ctr.FaultState{Rules: []ctr.PumbaRule{{RunID: "b", Chain: "FORWARD", Spec: "-A FORWARD -m comment --comment \"pumba:b\" -j DROP"}}},
		},
		{
			name:     "legacy warning but legacy command fails",
			outputs:  map[string]string{"tc qdisc show": "qdisc noqueue 0: dev eth0 root refcnt 2\n", "iptables -S": "-P INPUT ACCEPT\n"},
			stderrs:  map[string]string{"iptables -S": legacyWarning},
			failures: map[string]error{"iptables-legacy -S": errors.New("executable file not found")},
			wantErr:  "iptables-legacy tables present but listing them failed",
		},
		{
			name:    "legacy warning but legacy output unreadable",
			outputs: map[string]string{"tc qdisc show": "qdisc noqueue 0: dev eth0 root refcnt 2\n", "iptables -S": "-P INPUT ACCEPT\n", "iptables-legacy -S": "Permission denied\n"},
			stderrs: map[string]string{"iptables -S": legacyWarning},
			wantErr: "failed to parse iptables-legacy rules",
		},
		{name: "tc fails", failures: map[string]error{"tc qdisc show": errors.New("exit code 1")}, wantErr: "failed to list qdiscs"},
		{name: "tc output unreadable", outputs: map[string]string{"tc qdisc show": "Error: bad\n"}, wantErr: "failed to parse qdiscs"},
		{name: "iptables missing", failures: map[string]error{"iptables -S": errors.New("executable file not found")}, wantErr: "failed to list iptables rules"},
		{name: "iptables output unreadable", outputs: map[string]string{"iptables -S": "Permission denied\n"}, wantErr: "failed to parse iptables rules"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.outputs == nil {
				tt.outputs = map[string]string{}
			}
			tt.outputs["iptables --version"] = "iptables v1.8.13 (nf_tables)\n"
			got, err := Inspect(fakeRunner(tt.outputs, tt.stderrs, tt.failures))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIPTablesCommandsTranslatesRequest(t *testing.T) {
	_, src, _ := net.ParseCIDR("10.0.0.0/8")
	got := IPTablesCommands(&ctr.IPTablesRequest{
		CmdPrefix: []string{"-I", "INPUT"}, CmdSuffix: []string{"-j", "DROP"},
		SrcIPs: []*net.IPNet{src}, RunID: "r",
	})
	assert.Equal(t, [][]string{{"-I", "INPUT", "-s", "10.0.0.0/8", "-m", "comment", "--comment", "pumba:r", "-j", "DROP"}}, got)
}

func TestInspectBackendCoverage(t *testing.T) {
	nftRule := "-A INPUT -m comment --comment pumba:nft -j DROP\n"
	tests := []struct {
		name      string
		version   string
		nftOutput string
		failures  map[string]error
		wantErr   string
	}{
		{name: "legacy default finds nft rules", version: "iptables v1.8.13 (legacy)\n", nftOutput: nftRule},
		{name: "legacy default clean in both backends", version: "iptables v1.8.13 (legacy)\n", nftOutput: "-P INPUT ACCEPT\n"},
		{name: "legacy default missing nft tool", version: "iptables v1.8.13 (legacy)\n", failures: map[string]error{"iptables-nft -S": errors.New("not found")}, wantErr: "failed to list iptables-nft rules"},
		{name: "legacy default nft denied", version: "iptables v1.8.13 (legacy)\n", failures: map[string]error{"iptables-nft -S": errors.New("permission denied")}, wantErr: "failed to list iptables-nft rules"},
		{name: "legacy default unreadable nft output", version: "iptables v1.8.13 (legacy)\n", nftOutput: "Permission denied\n", wantErr: "failed to parse iptables-nft rules"},
		{name: "nft default needs no absent legacy tool", version: "iptables v1.8.13 (nf_tables)\n", failures: map[string]error{"iptables-legacy -S": errors.New("not found")}},
		{name: "version command fails", failures: map[string]error{"iptables --version": errors.New("not found")}, wantErr: "failed to identify iptables backend"},
		{name: "empty version", wantErr: "unrecognized iptables version"},
		{name: "old version without backend", version: "iptables v1.6.2\n", wantErr: "unrecognized iptables version"},
		{name: "unknown backend", version: "iptables v1.8.13 (unknown)\n", wantErr: "unrecognized iptables version"},
		{name: "malformed version", version: "not iptables (nf_tables)\n", wantErr: "unrecognized iptables version"},
		{name: "trailing garbage", version: "iptables v1.8.13 (nf_tables)\nPermission denied\n", wantErr: "unrecognized iptables version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs := map[string]string{"iptables --version": tt.version, "iptables -S": "-P INPUT ACCEPT\n", "iptables-nft -S": tt.nftOutput}
			var commands []string
			run := fakeRunner(outputs, nil, tt.failures)
			got, err := Inspect(func(argv []string) (string, string, error) {
				commands = append(commands, strings.Join(argv, " "))
				return run(argv)
			})
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			if tt.nftOutput == nftRule {
				assert.Equal(t, []ctr.PumbaRule{{RunID: "nft", Chain: "INPUT", Spec: strings.TrimSpace(nftRule)}}, got.Rules)
			} else {
				assert.Empty(t, got.Rules)
			}
			if strings.Contains(tt.version, "(legacy)") {
				assert.Contains(t, commands, "iptables-nft -S")
			} else {
				assert.NotContains(t, commands, "iptables-legacy -S")
			}
		})
	}
}
