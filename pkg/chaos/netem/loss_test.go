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

func TestNewLossCommand_Validation(t *testing.T) {
	mockClient := new(container.MockClient)
	gparams := &chaos.GlobalParams{Names: []string{"test"}}
	nparams := &Params{Iface: "eth0", Duration: time.Second}

	tests := []struct {
		name        string
		percent     float64
		correlation float64
		wantErr     string
	}{
		{
			name:    "valid minimal loss",
			percent: 10.0,
			wantErr: "",
		},
		{
			name:        "valid full params",
			percent:     20.0,
			correlation: 5.0,
			wantErr:     "",
		},
		{
			name:    "negative percent rejected",
			percent: -1.0,
			wantErr: "invalid loss percent",
		},
		{
			name:    "percent over 100 rejected",
			percent: 101.0,
			wantErr: "invalid loss percent",
		},
		{
			name:        "negative correlation rejected",
			percent:     10.0,
			correlation: -1.0,
			wantErr:     "invalid loss correlation",
		},
		{
			name:        "correlation over 100 rejected",
			percent:     10.0,
			correlation: 101.0,
			wantErr:     "invalid loss correlation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewLossCommand(mockClient, gparams, nparams,
				tt.percent, tt.correlation)
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
	mockClient := new(container.MockClient)
	gparams := &chaos.GlobalParams{Names: []string{"nonexistent"}}
	nparams := &Params{Iface: "eth0", Duration: time.Second}

	mockClient.On("ListContainers", mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{}, nil)

	cmd, err := NewLossCommand(mockClient, gparams, nparams, 10.0, 0.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestLossCommand_Run_DryRun(t *testing.T) {
	mockClient := new(container.MockClient)
	target := &container.Container{
		ContainerID:   "abc123",
		ContainerName: "target",
		Labels:        map[string]string{},
		Networks:      map[string]container.NetworkLink{},
	}
	gparams := &chaos.GlobalParams{Names: []string{"target"}, DryRun: true}
	nparams := &Params{Iface: "eth0", Duration: 100 * time.Millisecond, Image: "tc-image"}

	mockClient.On("ListContainers", mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{target}, nil)

	// loss 10.00%, correlation 5.00% → netem cmd: ["loss", "10.00", "5.00"]
	mockClient.On("NetemContainer", mock.Anything, target, "eth0",
		[]string{"loss", "10.00", "5.00"},
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "tc-image", false, true).
		Return(nil)

	mockClient.On("StopNetemContainer", mock.Anything, target, "eth0",
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		"tc-image", false, true).
		Return(nil)

	cmd, err := NewLossCommand(mockClient, gparams, nparams, 10.0, 5.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestLossCommand_Run_WithRandom(t *testing.T) {
	mockClient := new(container.MockClient)
	c1 := &container.Container{ContainerID: "id1", ContainerName: "c1"}
	c2 := &container.Container{ContainerID: "id2", ContainerName: "c2"}

	gparams := &chaos.GlobalParams{Names: []string{"c1", "c2"}, DryRun: true}
	nparams := &Params{Iface: "eth0", Duration: 100 * time.Millisecond, Image: "tc"}

	// Must match both containers initially
	mockClient.On("ListContainers", mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{c1, c2}, nil)

	// Since we use random=true, only one container will be selected.
	// We can't predict which one easily in the test without mocking RandomContainer,
	// but RandomContainer is a top-level function in container package.
	// For this unit test, we'll accept either c1 OR c2 being called, but not both.
	// However, stretchr/testify mocking is strict.
	// A better approach is to rely on ListNContainers logic if we could control it,
	// but here we just want to verify that runNetem is called for one container.

	// We will use a loose match for the container argument
	mockClient.On("NetemContainer", mock.Anything, mock.AnythingOfType("*container.Container"), "eth0",
		[]string{"loss", "10.00"},
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "tc", false, true).
		Return(nil).Once()

	mockClient.On("StopNetemContainer", mock.Anything, mock.AnythingOfType("*container.Container"), "eth0",
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		"tc", false, true).
		Return(nil).Once()

	cmd, err := NewLossCommand(mockClient, gparams, nparams, 10.0, 0.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), true)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}
