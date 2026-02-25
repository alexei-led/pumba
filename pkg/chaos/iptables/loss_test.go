package iptables

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewLossCommand_Validation(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gparams := &chaos.GlobalParams{Names: []string{"test"}}
	params := &Params{Iface: "eth0", Duration: time.Second}

	tests := []struct {
		name        string
		mode        string
		probability float64
		every       int
		packet      int
		wantErr     string
	}{
		{"valid random mode", ModeRandom, 0.5, 0, 0, ""},
		{"valid nth mode", ModeNTH, 0, 3, 0, ""},
		{"valid nth with packet", ModeNTH, 0, 5, 4, ""},
		{"invalid mode", "invalid", 0, 0, 0, "invalid loss mode"},
		{"random: negative probability", ModeRandom, -0.1, 0, 0, "invalid loss probability"},
		{"random: probability over 1", ModeRandom, 1.1, 0, 0, "invalid loss probability"},
		{"random: probability at boundary 0", ModeRandom, 0.0, 0, 0, ""},
		{"random: probability at boundary 1", ModeRandom, 1.0, 0, 0, ""},
		{"nth: zero every", ModeNTH, 0, 0, 0, "invalid loss every"},
		{"nth: negative every", ModeNTH, 0, -1, 0, "invalid loss every"},
		{"nth: negative packet", ModeNTH, 0, 3, -1, "invalid loss packet"},
		{"nth: packet equals every", ModeNTH, 0, 3, 3, "invalid loss packet"},
		{"nth: packet exceeds every-1", ModeNTH, 0, 3, 5, "invalid loss packet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewLossCommand(mockClient, gparams, params, tt.mode, tt.probability, tt.every, tt.packet)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, cmd)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cmd)
			}
		})
	}
}

func TestLossCommand_Run_NoContainers(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gparams := &chaos.GlobalParams{Names: []string{"nonexistent"}}
	params := &Params{Iface: "eth0", Duration: time.Second}

	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{}, nil)

	cmd, err := NewLossCommand(mockClient, gparams, params, ModeRandom, 0.5, 0, 0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
}

func TestLossCommand_Run_RandomMode(t *testing.T) {
	mockClient := container.NewMockClient(t)
	target := &container.Container{
		ContainerID:   "abc123",
		ContainerName: "target",
		Labels:        map[string]string{},
		Networks:      map[string]container.NetworkLink{},
	}
	gparams := &chaos.GlobalParams{Names: []string{"target"}, DryRun: true}
	params := &Params{Iface: "eth0", Protocol: "any", Duration: 100 * time.Millisecond, Image: "iptables-image"}

	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{target}, nil)

	addCmdPrefix := []string{"-I", "INPUT", "-i", "eth0"}
	delCmdPrefix := []string{"-D", "INPUT", "-i", "eth0"}
	cmdSuffix := []string{"-m", "statistic", "--mode", "random", "--probability", "0.50", "-j", "DROP"}

	mockClient.EXPECT().IPTablesContainer(mock.Anything, target,
		addCmdPrefix, cmdSuffix,
		([]*net.IPNet)(nil), ([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "iptables-image", false, true).
		Return(nil)

	mockClient.EXPECT().StopIPTablesContainer(mock.Anything, target,
		delCmdPrefix, cmdSuffix,
		([]*net.IPNet)(nil), ([]*net.IPNet)(nil), []string(nil), []string(nil),
		"iptables-image", false, true).
		Return(nil)

	cmd, err := NewLossCommand(mockClient, gparams, params, ModeRandom, 0.5, 0, 0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
}

func TestLossCommand_Run_NTHMode(t *testing.T) {
	mockClient := container.NewMockClient(t)
	target := &container.Container{
		ContainerID:   "abc123",
		ContainerName: "target",
		Labels:        map[string]string{},
		Networks:      map[string]container.NetworkLink{},
	}
	gparams := &chaos.GlobalParams{Names: []string{"target"}, DryRun: true}
	params := &Params{Iface: "eth0", Protocol: "any", Duration: 100 * time.Millisecond, Image: "iptables-image"}

	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{target}, nil)

	addCmdPrefix := []string{"-I", "INPUT", "-i", "eth0"}
	delCmdPrefix := []string{"-D", "INPUT", "-i", "eth0"}
	cmdSuffix := []string{"-m", "statistic", "--mode", "nth", "--every", "5", "--packet", "0", "-j", "DROP"}

	mockClient.EXPECT().IPTablesContainer(mock.Anything, target,
		addCmdPrefix, cmdSuffix,
		([]*net.IPNet)(nil), ([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "iptables-image", false, true).
		Return(nil)

	mockClient.EXPECT().StopIPTablesContainer(mock.Anything, target,
		delCmdPrefix, cmdSuffix,
		([]*net.IPNet)(nil), ([]*net.IPNet)(nil), []string(nil), []string(nil),
		"iptables-image", false, true).
		Return(nil)

	cmd, err := NewLossCommand(mockClient, gparams, params, ModeNTH, 0, 5, 0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
}

func TestLossCommand_Run_WithProtocol(t *testing.T) {
	mockClient := container.NewMockClient(t)
	target := &container.Container{
		ContainerID:   "abc123",
		ContainerName: "target",
		Labels:        map[string]string{},
		Networks:      map[string]container.NetworkLink{},
	}
	gparams := &chaos.GlobalParams{Names: []string{"target"}, DryRun: true}
	params := &Params{Iface: "eth0", Protocol: "tcp", Duration: 100 * time.Millisecond, Image: "iptables-image"}

	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{target}, nil)

	addCmdPrefix := []string{"-I", "INPUT", "-i", "eth0", "-p", "tcp"}
	delCmdPrefix := []string{"-D", "INPUT", "-i", "eth0", "-p", "tcp"}
	cmdSuffix := []string{"-m", "statistic", "--mode", "random", "--probability", "0.50", "-j", "DROP"}

	mockClient.EXPECT().IPTablesContainer(mock.Anything, target,
		addCmdPrefix, cmdSuffix,
		([]*net.IPNet)(nil), ([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "iptables-image", false, true).
		Return(nil)

	mockClient.EXPECT().StopIPTablesContainer(mock.Anything, target,
		delCmdPrefix, cmdSuffix,
		([]*net.IPNet)(nil), ([]*net.IPNet)(nil), []string(nil), []string(nil),
		"iptables-image", false, true).
		Return(nil)

	cmd, err := NewLossCommand(mockClient, gparams, params, ModeRandom, 0.5, 0, 0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
}

func TestLossCommand_Run_WithRandom(t *testing.T) {
	mockClient := container.NewMockClient(t)
	c1 := &container.Container{ContainerID: "id1", ContainerName: "c1"}
	c2 := &container.Container{ContainerID: "id2", ContainerName: "c2"}

	gparams := &chaos.GlobalParams{Names: []string{"c1", "c2"}, DryRun: true}
	params := &Params{Iface: "eth0", Protocol: "any", Duration: 100 * time.Millisecond, Image: "iptables-image"}

	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{c1, c2}, nil)

	addCmdPrefix := []string{"-I", "INPUT", "-i", "eth0"}
	delCmdPrefix := []string{"-D", "INPUT", "-i", "eth0"}
	cmdSuffix := []string{"-m", "statistic", "--mode", "random", "--probability", "0.50", "-j", "DROP"}

	mockClient.EXPECT().IPTablesContainer(mock.Anything, mock.AnythingOfType("*container.Container"),
		addCmdPrefix, cmdSuffix,
		([]*net.IPNet)(nil), ([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "iptables-image", false, true).
		Return(nil).Once()

	mockClient.EXPECT().StopIPTablesContainer(mock.Anything, mock.AnythingOfType("*container.Container"),
		delCmdPrefix, cmdSuffix,
		([]*net.IPNet)(nil), ([]*net.IPNet)(nil), []string(nil), []string(nil),
		"iptables-image", false, true).
		Return(nil).Once()

	cmd, err := NewLossCommand(mockClient, gparams, params, ModeRandom, 0.5, 0, 0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), true)
	assert.NoError(t, err)
}
