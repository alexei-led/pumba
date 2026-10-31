package netns

import (
	"errors"
	"net"
	"strings"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/tc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeRunner(outputs map[string]string, failures map[string]error) Runner {
	return func(argv []string) (string, error) {
		key := strings.Join(argv, " ")
		if err, ok := failures[key]; ok {
			return "", err
		}
		return outputs[key], nil
	}
}

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
		{name: "tc fails", failures: map[string]error{"tc qdisc show": errors.New("exit code 1")}, wantErr: "failed to list qdiscs"},
		{name: "tc output unreadable", outputs: map[string]string{"tc qdisc show": "Error: bad\n"}, wantErr: "failed to parse qdiscs"},
		{name: "iptables missing", failures: map[string]error{"iptables -S": errors.New("executable file not found")}, wantErr: "failed to list iptables rules"},
		{name: "iptables output unreadable", outputs: map[string]string{"iptables -S": "Permission denied\n"}, wantErr: "failed to parse iptables rules"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Inspect(fakeRunner(tt.outputs, tt.failures))
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
	assert.Equal(t, [][]string{{"iptables", "-I", "INPUT", "-s", "10.0.0.0/8", "-m", "comment", "--comment", "pumba:r", "-j", "DROP"}}, got)
}

func TestNetemCommandsTranslateRequest(t *testing.T) {
	_, dst, _ := net.ParseCIDR("10.0.0.0/8")
	req := &ctr.NetemRequest{Interface: "eth0", Command: []string{"delay", "100ms"}, IPs: []*net.IPNet{dst}, DPorts: []string{"80"}}
	want := &tc.NetemRequest{Interface: "eth0", Command: []string{"delay", "100ms"}, IPs: []string{"10.0.0.0/8"}, DPorts: []string{"80"}}

	assert.Equal(t, append([]string{"sh"}, tc.Start(want)...), NetemStart(req))
	assert.Equal(t, append([]string{"sh"}, tc.Stop(want)...), NetemStop(req))
}

func TestNetemPlan(t *testing.T) {
	tests := []struct {
		name      string
		sidecar   ctr.SidecarSpec
		wantVia   ctr.CommandVia
		wantImage string
	}{
		{name: "sidecar", sidecar: ctr.SidecarSpec{Image: "nettools", Pull: true}, wantVia: ctr.ViaSidecar, wantImage: "nettools"},
		{name: "exec in target", wantVia: ctr.ViaExec},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ctr.NetemRequest{Interface: "eth0", Command: []string{"loss", "10%"}, Sidecar: tt.sidecar}
			assert.Equal(t, &ctr.CommandPlan{
				Via: tt.wantVia, Image: tt.wantImage,
				Apply: [][]string{NetemStart(req)}, Revert: [][]string{NetemStop(req)},
			}, NetemPlan(req))
		})
	}
}

func TestIPTablesPlan(t *testing.T) {
	_, src, _ := net.ParseCIDR("10.0.0.1/32")
	_, dst, _ := net.ParseCIDR("10.0.0.2/32")
	tests := []struct {
		name       string
		filtered   bool
		sidecar    ctr.SidecarSpec
		wantVia    ctr.CommandVia
		wantImage  string
		wantApply  [][]string
		wantRevert [][]string
	}{
		{
			name: "unfiltered by exec", wantVia: ctr.ViaExec,
			wantApply:  [][]string{{"iptables", "-I", "INPUT", "-m", "comment", "--comment", "pumba:r1", "-j", "DROP"}},
			wantRevert: [][]string{{"iptables", "-D", "INPUT", "-m", "comment", "--comment", "pumba:r1", "-j", "DROP"}},
		},
		{
			name: "filtered in sidecar", filtered: true, sidecar: ctr.SidecarSpec{Image: "nettools"},
			wantVia: ctr.ViaSidecar, wantImage: "nettools",
			wantApply: [][]string{
				{"iptables", "-I", "INPUT", "-s", "10.0.0.1/32", "-m", "comment", "--comment", "pumba:r1", "-j", "DROP"},
				{"iptables", "-I", "INPUT", "-d", "10.0.0.2/32", "-m", "comment", "--comment", "pumba:r1", "-j", "DROP"},
			},
			wantRevert: [][]string{
				{"iptables", "-D", "INPUT", "-s", "10.0.0.1/32", "-m", "comment", "--comment", "pumba:r1", "-j", "DROP"},
				{"iptables", "-D", "INPUT", "-d", "10.0.0.2/32", "-m", "comment", "--comment", "pumba:r1", "-j", "DROP"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			add := &ctr.IPTablesRequest{CmdPrefix: []string{"-I", "INPUT"}, CmdSuffix: []string{"-j", "DROP"}, Sidecar: tt.sidecar, RunID: "r1"}
			if tt.filtered {
				add.SrcIPs, add.DstIPs = []*net.IPNet{src}, []*net.IPNet{dst}
			}
			del := *add
			del.CmdPrefix = []string{"-D", "INPUT"}
			assert.Equal(t, &ctr.CommandPlan{Via: tt.wantVia, Image: tt.wantImage, Apply: tt.wantApply, Revert: tt.wantRevert}, IPTablesPlan(add, &del))
		})
	}
}
