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

func TestNewDelayCommand_Validation(t *testing.T) {
	mockClient := new(container.MockClient)
	gparams := &chaos.GlobalParams{Names: []string{"test"}}
	nparams := &Params{Iface: "eth0", Duration: time.Second}

	tests := []struct {
		name         string
		delay        int
		jitter       int
		correlation  float64
		distribution string
		wantErr      string
	}{
		{
			name:    "valid minimal delay",
			delay:   100,
			wantErr: "",
		},
		{
			name:         "valid full params",
			delay:        500,
			jitter:       100,
			correlation:  25.0,
			distribution: "normal",
			wantErr:      "",
		},
		{
			name:    "zero delay rejected",
			delay:   0,
			wantErr: "non-positive delay time",
		},
		{
			name:    "negative delay rejected",
			delay:   -1,
			wantErr: "non-positive delay time",
		},
		{
			name:    "negative jitter rejected",
			delay:   100,
			jitter:  -1,
			wantErr: "invalid delay jitter",
		},
		{
			name:    "jitter exceeds delay rejected",
			delay:   100,
			jitter:  200,
			wantErr: "invalid delay jitter",
		},
		{
			name:        "negative correlation rejected",
			delay:       100,
			correlation: -1.0,
			wantErr:     "invalid delay correlation",
		},
		{
			name:        "correlation over 100 rejected",
			delay:       100,
			correlation: 101.0,
			wantErr:     "invalid delay correlation",
		},
		{
			name:         "invalid distribution rejected",
			delay:        100,
			distribution: "gaussian",
			wantErr:      "invalid delay distribution",
		},
		{
			name:         "all valid distributions",
			delay:        100,
			distribution: "uniform",
			wantErr:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewDelayCommand(mockClient, gparams, nparams,
				tt.delay, tt.jitter, tt.correlation, tt.distribution)
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

func TestDelayCommand_Run_NoContainers(t *testing.T) {
	mockClient := new(container.MockClient)
	gparams := &chaos.GlobalParams{Names: []string{"nonexistent"}}
	nparams := &Params{Iface: "eth0", Duration: time.Second}

	mockClient.On("ListContainers", mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{}, nil)

	cmd, err := NewDelayCommand(mockClient, gparams, nparams, 100, 0, 0, "")
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestDelayCommand_Run_DryRun(t *testing.T) {
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

	// delay 200ms, jitter 50ms, correlation 25.50% → netem cmd: ["delay", "200ms", "50ms", "25.50"]
	mockClient.On("NetemContainer", mock.Anything, target, "eth0",
		[]string{"delay", "200ms", "50ms", "25.50"},
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "tc-image", false, true).
		Return(nil)

	mockClient.On("StopNetemContainer", mock.Anything, target, "eth0",
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		"tc-image", false, true).
		Return(nil)

	cmd, err := NewDelayCommand(mockClient, gparams, nparams, 200, 50, 25.5, "")
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestDelayCommand_Run_WithDistribution(t *testing.T) {
	mockClient := new(container.MockClient)
	target := &container.Container{
		ContainerID:   "abc123",
		ContainerName: "target",
		Labels:        map[string]string{},
		Networks:      map[string]container.NetworkLink{},
	}
	gparams := &chaos.GlobalParams{Names: []string{"target"}, DryRun: true}
	nparams := &Params{Iface: "eth0", Duration: 100 * time.Millisecond, Image: "tc"}

	mockClient.On("ListContainers", mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{target}, nil)

	// delay 100ms, jitter 20ms, no correlation, distribution normal
	// → ["delay", "100ms", "20ms", "distribution", "normal"]
	mockClient.On("NetemContainer", mock.Anything, target, "eth0",
		[]string{"delay", "100ms", "20ms", "distribution", "normal"},
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "tc", false, true).
		Return(nil)

	mockClient.On("StopNetemContainer", mock.Anything, target, "eth0",
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		"tc", false, true).
		Return(nil)

	cmd, err := NewDelayCommand(mockClient, gparams, nparams, 100, 20, 0, "normal")
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}
