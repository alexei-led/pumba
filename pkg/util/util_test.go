package util

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPorts(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"empty string", "", nil, false},
		{"single valid port", "80", []string{"80"}, false},
		{"multiple valid ports", "80,443,8080", []string{"80", "443", "8080"}, false},
		{"boundary port zero", "0", []string{"0"}, false},
		{"boundary port 65535", "65535", []string{"65535"}, false},
		{"port above max", "65536", nil, true},
		{"negative port", "-1", nil, true},
		{"non-numeric port", "abc", nil, true},
		{"invalid port in list", "80,abc,443", nil, true},
		{"port way out of range", "100000", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetPorts(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid port specified")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVerifyPort(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		wantErr bool
	}{
		{"empty string", "", false},
		{"zero", "0", false},
		{"max valid", "65535", false},
		{"mid range", "8080", false},
		{"above max", "65536", true},
		{"negative", "-1", true},
		{"non-numeric", "not-a-port", true},
		{"float", "80.5", true},
		{"leading zeros", "0080", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyPort(tt.port)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCidrNotation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain IP gets /32", "10.0.0.1", "10.0.0.1/32"},
		{"existing CIDR unchanged", "10.0.0.0/24", "10.0.0.0/24"},
		{"existing /32 unchanged", "192.168.1.1/32", "192.168.1.1/32"},
		{"IPv6-like with slash", "::1/128", "::1/128"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cidrNotation(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseCIDR(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNet *net.IPNet
		wantErr bool
	}{
		{
			"plain IPv4 auto /32",
			"10.0.0.1",
			&net.IPNet{
				IP:   net.IPv4(10, 0, 0, 1).To4(),
				Mask: net.CIDRMask(32, 32),
			},
			false,
		},
		{
			"CIDR notation /24",
			"192.168.1.0/24",
			&net.IPNet{
				IP:   net.IPv4(192, 168, 1, 0).To4(),
				Mask: net.CIDRMask(24, 32),
			},
			false,
		},
		{
			"CIDR notation /16",
			"10.0.0.0/16",
			&net.IPNet{
				IP:   net.IPv4(10, 0, 0, 0).To4(),
				Mask: net.CIDRMask(16, 32),
			},
			false,
		},
		{"invalid IP", "not-an-ip", nil, true},
		{"empty string", "", nil, true},
		{"invalid CIDR mask", "10.0.0.1/33", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCIDR(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to parse CIDR")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNet, got)
		})
	}
}
