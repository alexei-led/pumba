package container

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSidecarSpec_ZeroValueIsSafe(t *testing.T) {
	var s SidecarSpec
	assert.Empty(t, s.Image)
	assert.False(t, s.Pull)
}

func TestSidecarSpec_Hydration(t *testing.T) {
	s := SidecarSpec{Image: "ghcr.io/alexei-led/pumba-alpine-nettools:latest", Pull: true}
	assert.Equal(t, "ghcr.io/alexei-led/pumba-alpine-nettools:latest", s.Image)
	assert.True(t, s.Pull)
}

func TestNetemRequest_ZeroValueIsSafe(t *testing.T) {
	var r NetemRequest
	assert.Nil(t, r.Container)
	assert.Empty(t, r.Interface)
	assert.Nil(t, r.Command)
	assert.Nil(t, r.IPs)
	assert.Nil(t, r.SPorts)
	assert.Nil(t, r.DPorts)
	assert.Equal(t, time.Duration(0), r.Duration)
	assert.Equal(t, SidecarSpec{}, r.Sidecar)
	assert.False(t, r.DryRun)
}

func TestNetemRequest_Hydration(t *testing.T) {
	c := CreateTestContainers(1)[0]
	_, ipNet, _ := net.ParseCIDR("10.0.0.0/24")
	r := NetemRequest{
		Container: c,
		Interface: "eth0",
		Command:   []string{"delay", "100ms"},
		IPs:       []*net.IPNet{ipNet},
		SPorts:    []string{"80"},
		DPorts:    []string{"443"},
		Duration:  5 * time.Second,
		Sidecar:   SidecarSpec{Image: "img", Pull: true},
		DryRun:    true,
	}
	assert.Same(t, c, r.Container)
	assert.Equal(t, "eth0", r.Interface)
	assert.Equal(t, []string{"delay", "100ms"}, r.Command)
	assert.Equal(t, []*net.IPNet{ipNet}, r.IPs)
	assert.Equal(t, []string{"80"}, r.SPorts)
	assert.Equal(t, []string{"443"}, r.DPorts)
	assert.Equal(t, 5*time.Second, r.Duration)
	assert.Equal(t, SidecarSpec{Image: "img", Pull: true}, r.Sidecar)
	assert.True(t, r.DryRun)
}

func TestIPTablesRequest_ZeroValueIsSafe(t *testing.T) {
	var r IPTablesRequest
	assert.Nil(t, r.Container)
	assert.Nil(t, r.CmdPrefix)
	assert.Nil(t, r.CmdSuffix)
	assert.Nil(t, r.SrcIPs)
	assert.Nil(t, r.DstIPs)
	assert.Nil(t, r.SPorts)
	assert.Nil(t, r.DPorts)
	assert.Equal(t, time.Duration(0), r.Duration)
	assert.Equal(t, SidecarSpec{}, r.Sidecar)
	assert.False(t, r.DryRun)
}

func TestIPTablesRequest_Hydration(t *testing.T) {
	c := CreateTestContainers(1)[0]
	_, src, _ := net.ParseCIDR("10.0.0.0/24")
	_, dst, _ := net.ParseCIDR("192.168.0.0/16")
	r := IPTablesRequest{
		Container: c,
		CmdPrefix: []string{"-A", "INPUT"},
		CmdSuffix: []string{"-j", "DROP"},
		SrcIPs:    []*net.IPNet{src},
		DstIPs:    []*net.IPNet{dst},
		SPorts:    []string{"80"},
		DPorts:    []string{"443"},
		Duration:  5 * time.Second,
		Sidecar:   SidecarSpec{Image: "img", Pull: true},
		DryRun:    true,
	}
	assert.Same(t, c, r.Container)
	assert.Equal(t, []string{"-A", "INPUT"}, r.CmdPrefix)
	assert.Equal(t, []string{"-j", "DROP"}, r.CmdSuffix)
	assert.Equal(t, []*net.IPNet{src}, r.SrcIPs)
	assert.Equal(t, []*net.IPNet{dst}, r.DstIPs)
	assert.Equal(t, []string{"80"}, r.SPorts)
	assert.Equal(t, []string{"443"}, r.DPorts)
	assert.Equal(t, 5*time.Second, r.Duration)
	assert.Equal(t, SidecarSpec{Image: "img", Pull: true}, r.Sidecar)
	assert.True(t, r.DryRun)
}
