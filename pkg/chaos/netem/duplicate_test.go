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

func TestNewDuplicateCommand_Validation(t *testing.T) {
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
			name:    "valid minimal duplicate",
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
			wantErr: "invalid duplicate percent",
		},
		{
			name:    "percent over 100 rejected",
			percent: 101.0,
			wantErr: "invalid duplicate percent",
		},
		{
			name:        "negative correlation rejected",
			percent:     10.0,
			correlation: -1.0,
			wantErr:     "invalid duplicate correlation",
		},
		{
			name:        "correlation over 100 rejected",
			percent:     10.0,
			correlation: 101.0,
			wantErr:     "invalid duplicate correlation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewDuplicateCommand(mockClient, gparams, nparams,
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

func TestDuplicateCommand_Run_NoContainers(t *testing.T) {
	mockClient := new(container.MockClient)
	gparams := &chaos.GlobalParams{Names: []string{"nonexistent"}}
	nparams := &Params{Iface: "eth0", Duration: time.Second}

	mockClient.On("ListContainers", mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{}, nil)

	cmd, err := NewDuplicateCommand(mockClient, gparams, nparams, 10.0, 0.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestDuplicateCommand_Run_DryRun(t *testing.T) {
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

	// duplicate 10.00%, correlation 5.00% → netem cmd: ["duplicate", "10.00", "5.00"]
	mockClient.On("NetemContainer", mock.Anything, target, "eth0",
		[]string{"duplicate", "10.00", "5.00"},
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "tc-image", false, true).
		Return(nil)

	mockClient.On("StopNetemContainer", mock.Anything, target, "eth0",
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		"tc-image", false, true).
		Return(nil)

	cmd, err := NewDuplicateCommand(mockClient, gparams, nparams, 10.0, 5.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}
