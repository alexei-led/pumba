package netem

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

func TestLossStateCommand_Run_NoContainers(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gparams := &chaos.GlobalParams{Names: []string{"nonexistent"}}
	nparams := &Params{Iface: "eth0", Duration: time.Second}

	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{}, nil)

	cmd, err := NewLossStateCommand(mockClient, gparams, nparams, 10.0, 20.0, 30.0, 40.0, 50.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
}

func TestLossStateCommand_Run_DryRun(t *testing.T) {
	mockClient := container.NewMockClient(t)
	target := &container.Container{
		ContainerID:   "abc123",
		ContainerName: "target",
		Labels:        map[string]string{},
		Networks:      map[string]container.NetworkLink{},
	}
	gparams := &chaos.GlobalParams{Names: []string{"target"}, DryRun: true}
	nparams := &Params{Iface: "eth0", Duration: 100 * time.Millisecond, Image: "tc-image"}

	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{target}, nil)

	mockClient.EXPECT().NetemContainer(mock.Anything, target, "eth0",
		[]string{"loss", "state", "10.00", "20.00", "30.00", "40.00", "50.00"},
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "tc-image", false, true).
		Return(nil)

	mockClient.EXPECT().StopNetemContainer(mock.Anything, target, "eth0",
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		"tc-image", false, true).
		Return(nil)

	cmd, err := NewLossStateCommand(mockClient, gparams, nparams, 10.0, 20.0, 30.0, 40.0, 50.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
}

func TestLossStateCommand_Run_WithRandom(t *testing.T) {
	mockClient := container.NewMockClient(t)
	c1 := &container.Container{ContainerID: "id1", ContainerName: "c1"}
	c2 := &container.Container{ContainerID: "id2", ContainerName: "c2"}

	gparams := &chaos.GlobalParams{Names: []string{"c1", "c2"}, DryRun: true}
	nparams := &Params{Iface: "eth0", Duration: 100 * time.Millisecond, Image: "tc"}

	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{c1, c2}, nil)

	mockClient.EXPECT().NetemContainer(mock.Anything, mock.AnythingOfType("*container.Container"), "eth0",
		[]string{"loss", "state", "10.00", "20.00", "30.00", "40.00", "50.00"},
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "tc", false, true).
		Return(nil).Once()

	mockClient.EXPECT().StopNetemContainer(mock.Anything, mock.AnythingOfType("*container.Container"), "eth0",
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		"tc", false, true).
		Return(nil).Once()

	cmd, err := NewLossStateCommand(mockClient, gparams, nparams, 10.0, 20.0, 30.0, 40.0, 50.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), true)
	assert.NoError(t, err)
}
