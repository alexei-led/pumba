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

func TestNewDelayCommand_Validation(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gparams := &chaos.GlobalParams{Names: []string{"test"}}
	nparams := &container.NetemRequest{Interface: "eth0", Duration: time.Second}

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
			cmd, err := NewDelayCommand(mockClient, gparams, nparams, 0,
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
	mockClient := container.NewMockClient(t)
	gparams := &chaos.GlobalParams{Names: []string{"nonexistent"}}
	nparams := &container.NetemRequest{Interface: "eth0", Duration: time.Second}

	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{}, nil)

	cmd, err := NewDelayCommand(mockClient, gparams, nparams, 0, 100, 0, 0, "")
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestDelayCommand_Run_WithRandom(t *testing.T) {
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

	cmd, err := NewDelayCommand(mockClient, gparams, nparams, 0, 100, 0, 0, "")
	require.NoError(t, err)

	err = cmd.Run(context.Background(), true)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestDelayCommand_Run_DryRun(t *testing.T) {
	tests := []struct {
		name        string
		delay       int
		jitter      int
		correlation float64
		dist        string
		image       string
		netemCmd    []string
	}{
		{
			name:        "delay with jitter and correlation",
			delay:       200,
			jitter:      50,
			correlation: 25.5,
			dist:        "",
			image:       "tc-image",
			netemCmd:    []string{"delay", "200ms", "50ms", "25.50"},
		},
		{
			name:     "delay with distribution",
			delay:    100,
			jitter:   20,
			dist:     "normal",
			image:    "tc",
			netemCmd: []string{"delay", "100ms", "20ms", "distribution", "normal"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				Sidecar:   container.SidecarSpec{Image: tt.image},
				DryRun:    true,
			}

			mockClient.EXPECT().ListContainers(mock.Anything,
				mock.AnythingOfType("container.FilterFunc"),
				container.ListOpts{All: false, Labels: nil}).
				Return([]*container.Container{target}, nil)

			expectedReq := &container.NetemRequest{
				Container: target,
				Interface: "eth0",
				Command:   tt.netemCmd,
				Duration:  100 * time.Millisecond,
				Sidecar:   container.SidecarSpec{Image: tt.image},
				DryRun:    true,
			}
			mockClient.EXPECT().NetemContainer(mock.Anything, expectedReq).Return(nil)
			mockClient.EXPECT().StopNetemContainer(mock.Anything, expectedReq).Return(nil)

			cmd, err := NewDelayCommand(mockClient, gparams, nparams, 0, tt.delay, tt.jitter, tt.correlation, tt.dist)
			require.NoError(t, err)

			err = cmd.Run(context.Background(), false)
			assert.NoError(t, err)
			mockClient.AssertExpectations(t)
		})
	}
}
