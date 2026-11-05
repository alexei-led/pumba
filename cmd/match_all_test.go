package main

import (
	"net"
	"strings"
	"testing"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMatchAllIPTablesCLIPlan(t *testing.T) {
	_, src, err := net.ParseCIDR("192.0.2.1/32")
	require.NoError(t, err)
	web := &ctr.Container{ContainerID: "web-id", ContainerName: "web", State: ctr.StateRunning}
	add := &ctr.IPTablesRequest{
		Container: web, CmdPrefix: []string{"-I", "INPUT", "-i", "eth0", "-p", "tcp"},
		CmdSuffix: []string{"-m", "statistic", "--mode", "random", "--probability", "1.00", "-j", "DROP"},
		SrcIPs:    []*net.IPNet{src}, DstIPs: []*net.IPNet{}, DPorts: []string{"443"},
		Duration: time.Hour, Sidecar: ctr.SidecarSpec{Pull: true}, DryRun: true, RunID: "r1", MatchAll: true,
	}
	del := *add
	del.CmdPrefix = []string{"-D", "INPUT", "-i", "eth0", "-p", "tcp"}
	client := ctr.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), ctr.ListOpts{}).Return([]*ctr.Container{web}, nil).Once()
	client.EXPECT().PlanIPTablesContainer(add, &del).RunAndReturn(netns.IPTablesPlan).Once()
	client.EXPECT().Close().Return(nil).Maybe()
	code, stdout, _ := runPumba(t, client, "--dry-run", "--run-id", "r1",
		"iptables", "--duration", "1h", "--iptables-image", "", "--protocol", "tcp",
		"--match-all", "--source", "192.0.2.1", "--dst-port", "443", "loss", "--probability", "1", "web")
	require.Equal(t, 0, code)
	assert.Equal(t, 1, strings.Count(stdout, "\n  apply:"))
	assert.Contains(t, stdout, "-s 192.0.2.1/32 --dport 443")
}

func TestMatchAllNetemCLIPlan(t *testing.T) {
	_, dst, err := net.ParseCIDR("192.0.2.1/32")
	require.NoError(t, err)
	web := &ctr.Container{ContainerID: "web-id", ContainerName: "web", State: ctr.StateRunning}
	req := &ctr.NetemRequest{
		Container: web, Interface: "eth0", Command: []string{"delay", "100ms"},
		IPs: []*net.IPNet{dst}, DPorts: []string{"443"}, MatchAll: true,
		Duration: time.Hour, Sidecar: ctr.SidecarSpec{Pull: true}, DryRun: true, RunID: "r1",
	}
	client := ctr.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), ctr.ListOpts{}).Return([]*ctr.Container{web}, nil).Once()
	client.EXPECT().PlanNetemContainer(req).RunAndReturn(netns.NetemPlan).Once()
	client.EXPECT().Close().Return(nil).Maybe()
	code, stdout, _ := runPumba(t, client, "--dry-run", "--run-id", "r1",
		"netem", "--duration", "1h", "--tc-image", "", "--match-all", "--target", "192.0.2.1", "--ingress-port", "443",
		"delay", "--time", "100", "--jitter", "0", "--correlation", "0", "web")
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, "match ip dst")
	assert.Contains(t, stdout, "match ip dport")
	assert.Contains(t, stdout, "filter groups")
}
