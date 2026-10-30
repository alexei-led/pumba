package tc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	loNoqueue = "qdisc noqueue 0: dev lo root refcnt 2 \n"
	prioRoot  = "qdisc prio 504d: dev eth0 root refcnt 2 bands 3 priomap 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1\n"
	sfqBand1  = "qdisc sfq 504e: dev eth0 parent 504d:1 limit 127p quantum 1514b depth 127 divisor 1024 \n"
	sfqBand2  = "qdisc sfq 504f: dev eth0 parent 504d:2 limit 127p quantum 1514b depth 127 divisor 1024 \n"
	netemBand = "qdisc netem 5050: dev eth0 parent 504d:3 limit 1000 delay 100ms\n"
)

func TestParseQdiscs(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []PumbaQdisc
	}{
		{name: "empty output", output: ""},
		{name: "noqueue default root", output: loNoqueue + "qdisc noqueue 0: dev eth0 root refcnt 2 \n"},
		{name: "fq_codel and mq defaults", output: "qdisc mq 0: dev eth0 root \nqdisc fq_codel 0: dev eth0 parent :1 limit 10240p flows 1024\n"},
		{
			name:   "netem root",
			output: loNoqueue + "qdisc netem 504d: dev eth0 root refcnt 2 limit 1000 delay 100ms  10ms\n",
			want:   []PumbaQdisc{{Device: "eth0", Kind: "netem", Ownership: OwnershipPumba}},
		},
		{
			name:   "prio sfq netem scoped topology",
			output: loNoqueue + prioRoot + sfqBand1 + sfqBand2 + netemBand,
			want:   []PumbaQdisc{{Device: "eth0", Kind: "prio", Ownership: OwnershipPumba}},
		},
		{name: "foreign root", output: "qdisc htb 1: dev eth0 root refcnt 2 r2q 10 default 0x30 direct_packets_stat 0\n"},
		{
			name:   "prio root missing a band",
			output: prioRoot + sfqBand1 + netemBand,
			want:   []PumbaQdisc{{Device: "eth0", Kind: "prio", Ownership: OwnershipSuspicious}},
		},
		{
			name:   "prio root with wrong band kind",
			output: prioRoot + sfqBand1 + sfqBand2 + "qdisc tbf 5050: dev eth0 parent 504d:3 rate 1Mbit burst 32Kb lat 400ms\n",
			want:   []PumbaQdisc{{Device: "eth0", Kind: "prio", Ownership: OwnershipSuspicious}},
		},
		{
			name:   "netem root with a child",
			output: "qdisc netem 504d: dev eth0 root refcnt 2 limit 1000 delay 1s\nqdisc pfifo 1: dev eth0 parent 504d:1 limit 1000p\n",
			want:   []PumbaQdisc{{Device: "eth0", Kind: "netem", Ownership: OwnershipSuspicious}},
		},
		{
			name:   "pumba handle under a foreign root",
			output: "qdisc htb 1: dev eth0 root refcnt 2\nqdisc netem 504d: dev eth0 parent 1:10 limit 1000 delay 1s\n",
			want:   []PumbaQdisc{{Device: "eth0", Kind: "netem", Ownership: OwnershipSuspicious}},
		},
		{
			name:   "tbf root with pumba handle",
			output: "qdisc tbf 504d: dev eth0 root refcnt 2 rate 1Mbit burst 32Kb lat 400ms\n",
			want:   []PumbaQdisc{{Device: "eth0", Kind: "tbf", Ownership: OwnershipSuspicious}},
		},
		{
			name: "multiple devices",
			output: loNoqueue + prioRoot + sfqBand1 + sfqBand2 + netemBand +
				"qdisc netem 504d: dev eth1 root refcnt 2 limit 1000 loss 50%\n" +
				"qdisc noqueue 0: dev eth2 root refcnt 2\n" +
				"qdisc ingress ffff: dev eth2 parent ffff:fff1 ----------------\n",
			want: []PumbaQdisc{
				{Device: "eth0", Kind: "prio", Ownership: OwnershipPumba},
				{Device: "eth1", Kind: "netem", Ownership: OwnershipPumba},
			},
		},
		{
			name:   "indented continuation lines are ignored",
			output: "qdisc netem 504d: dev eth0 root refcnt 2 limit 1000 delay 1s\n  Sent 0 bytes 0 pkt\n",
			want:   []PumbaQdisc{{Device: "eth0", Kind: "netem", Ownership: OwnershipPumba}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseQdiscs(tt.output)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseQdiscsRejectsUnreadableOutput(t *testing.T) {
	for name, output := range map[string]string{
		"error text":        "Cannot find device \"eth9\"\n",
		"missing device":    "qdisc netem 504d: root refcnt 2\n",
		"truncated":         "qdisc netem\n",
		"dangling dev word": "qdisc netem 504d: root dev\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseQdiscs(output)
			require.Error(t, err)
		})
	}
}

func TestParseTaggedRules(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []TaggedRule
	}{
		{name: "empty output", output: ""},
		{name: "policies only", output: "-P INPUT ACCEPT\n-P FORWARD ACCEPT\n-P OUTPUT ACCEPT\n"},
		{
			name:   "untagged and foreign comments are skipped",
			output: "-P INPUT ACCEPT\n-A INPUT -i eth0 -j DROP\n-A INPUT -m comment --comment \"other:x\" -j ACCEPT\n-A INPUT -m comment --comment pumbaX -j ACCEPT\n",
		},
		{
			name: "quoted comments",
			output: "-P INPUT ACCEPT\n" +
				"-A INPUT -i eth0 -p tcp -m comment --comment \"pumba:3f9c2a7b1d04\" -m statistic --mode random --probability 0.20000000000 -j DROP\n" +
				"-A INPUT -s 10.0.0.1/32 -i eth0 -m comment --comment \"pumba:run.1\" -j DROP\n",
			want: []TaggedRule{
				{RunID: "3f9c2a7b1d04", Chain: "INPUT", Spec: "-A INPUT -i eth0 -p tcp -m comment --comment \"pumba:3f9c2a7b1d04\" -m statistic --mode random --probability 0.20000000000 -j DROP"},
				{RunID: "run.1", Chain: "INPUT", Spec: "-A INPUT -s 10.0.0.1/32 -i eth0 -m comment --comment \"pumba:run.1\" -j DROP"},
			},
		},
		{
			name:   "escaped quote inside comment",
			output: `-A OUTPUT -m comment --comment "pumba:a\"b c" -j DROP` + "\n",
			want:   []TaggedRule{{RunID: `a"b c`, Chain: "OUTPUT", Spec: `-A OUTPUT -m comment --comment "pumba:a\"b c" -j DROP`}},
		},
		{
			name:   "legacy warning line carries no rule",
			output: "-N PUMBA\n-A PUMBA -m comment --comment \"pumba:r\" -j DROP\n# Warning: iptables-legacy tables present, use iptables-legacy to see them\n",
			want:   []TaggedRule{{RunID: "r", Chain: "PUMBA", Spec: "-A PUMBA -m comment --comment \"pumba:r\" -j DROP"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTaggedRules(tt.output)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Rules)
		})
	}
}

func TestParseTaggedRulesReportsLegacyTables(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{name: "no warning", output: "-P INPUT ACCEPT\n", want: false},
		{name: "other comment", output: "# Generated by iptables-save\n-P INPUT ACCEPT\n", want: false},
		{name: "warning", output: "-P INPUT ACCEPT\n# Warning: iptables-legacy tables present, use iptables-legacy to see them\n", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTaggedRules(tt.output)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.LegacyTablesPresent)
		})
	}
}

func TestLegacyTablesPresent(t *testing.T) {
	assert.True(t, LegacyTablesPresent("# Warning: iptables-legacy tables present, use iptables-legacy to see them\n"))
	assert.False(t, LegacyTablesPresent("-P INPUT ACCEPT\n"))
}

func TestParseTaggedRulesRejectsUnreadableOutput(t *testing.T) {
	for name, output := range map[string]string{
		"error text":         "iptables v1.8.9 (legacy): can't initialize iptables table `filter': Permission denied\n",
		"unterminated quote": "-A INPUT -m comment --comment \"pumba:x -j DROP\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseTaggedRules(output)
			require.Error(t, err)
		})
	}
}
