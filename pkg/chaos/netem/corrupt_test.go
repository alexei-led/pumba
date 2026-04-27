package netem

import (
	"context"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCorruptCommand_Run_NoContainers(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gparams := &chaos.GlobalParams{Names: []string{"nonexistent"}}
	nparams := &container.NetemRequest{Interface: "eth0", Duration: time.Second}

	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{}, nil)

	cmd, err := NewCorruptCommand(mockClient, gparams, nparams, 0, 10.0, 0.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCorruptCommand_Run_WithRandom(t *testing.T) {
	mockClient := container.NewMockClient(t)
	c1 := &container.Container{ContainerID: "id1", ContainerName: "c1"}
	c2 := &container.Container{ContainerID: "id2", ContainerName: "c2"}

	gparams := &chaos.GlobalParams{Names: []string{"c1", "c2"}, DryRun: true}
	nparams := &container.NetemRequest{
		Interface: "eth0",
		Duration:  100 * time.Millisecond,
		Sidecar:   container.SidecarSpec{Image: "tc"},
		DryRun:    true,
	}

	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{c1, c2}, nil)

	mockClient.EXPECT().NetemContainer(mock.Anything, mock.AnythingOfType("*container.NetemRequest")).Return(nil).Once()
	mockClient.EXPECT().StopNetemContainer(mock.Anything, mock.AnythingOfType("*container.NetemRequest")).Return(nil).Once()

	cmd, err := NewCorruptCommand(mockClient, gparams, nparams, 0, 10.0, 0.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), true)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCorruptCommand_Run_DryRun(t *testing.T) {
	mockClient := container.NewMockClient(t)
	target := &container.Container{
		ContainerID:   "abc123",
		ContainerName: "target",
		Labels:        map[string]string{},
		Networks:      map[string]container.NetworkLink{},
	}
	gparams := &chaos.GlobalParams{Names: []string{"target"}, DryRun: true}
	nparams := &container.NetemRequest{
		Interface: "eth0",
		Duration:  100 * time.Millisecond,
		Sidecar:   container.SidecarSpec{Image: "tc-image"},
		DryRun:    true,
	}

	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{target}, nil)

	expectedReq := &container.NetemRequest{
		Container: target,
		Interface: "eth0",
		Command:   []string{"corrupt", "10.00", "5.00"},
		Duration:  100 * time.Millisecond,
		Sidecar:   container.SidecarSpec{Image: "tc-image"},
		DryRun:    true,
	}
	mockClient.EXPECT().NetemContainer(mock.Anything, expectedReq).Return(nil)
	mockClient.EXPECT().StopNetemContainer(mock.Anything, expectedReq).Return(nil)

	cmd, err := NewCorruptCommand(mockClient, gparams, nparams, 0, 10.0, 5.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}
