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

func TestCorruptCommand_Run_DryRun(t *testing.T) {
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

	cmd, err := NewCorruptCommand(mockClient, gparams, nparams, 10.0, 5.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}
