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

func TestNewLossGECommand_Validation(t *testing.T) {
	mockClient := new(container.MockClient)
	gparams := &chaos.GlobalParams{Names: []string{"test"}}
	nparams := &Params{Iface: "eth0", Duration: time.Second}

	tests := []struct {
		name    string
		pg      float64
		pb      float64
		oneH    float64
		oneK    float64
		wantErr string
	}{
		{
			name: "valid params",
			pg:   50.0,
			pb:   50.0,
			oneH: 50.0,
			oneK: 50.0,
		},
		{
			name:    "negative pg rejected",
			pg:      -1.0,
			pb:      50.0,
			oneH:    50.0,
			oneK:    50.0,
			wantErr: "invalid pg",
		},
		{
			name:    "pg over 100 rejected",
			pg:      101.0,
			pb:      50.0,
			oneH:    50.0,
			oneK:    50.0,
			wantErr: "invalid pg",
		},
		{
			name:    "negative pb rejected",
			pg:      50.0,
			pb:      -1.0,
			oneH:    50.0,
			oneK:    50.0,
			wantErr: "invalid pb",
		},
		{
			name:    "pb over 100 rejected",
			pg:      50.0,
			pb:      101.0,
			oneH:    50.0,
			oneK:    50.0,
			wantErr: "invalid pb",
		},
		{
			name:    "negative oneH rejected",
			pg:      50.0,
			pb:      50.0,
			oneH:    -1.0,
			oneK:    50.0,
			wantErr: "invalid loss probability",
		},
		{
			name:    "oneH over 100 rejected",
			pg:      50.0,
			pb:      50.0,
			oneH:    101.0,
			oneK:    50.0,
			wantErr: "invalid loss probability",
		},
		{
			name:    "negative oneK rejected",
			pg:      50.0,
			pb:      50.0,
			oneH:    50.0,
			oneK:    -1.0,
			wantErr: "invalid loss probability",
		},
		{
			name:    "oneK over 100 rejected",
			pg:      50.0,
			pb:      50.0,
			oneH:    50.0,
			oneK:    101.0,
			wantErr: "invalid loss probability",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewLossGECommand(mockClient, gparams, nparams,
				tt.pg, tt.pb, tt.oneH, tt.oneK)
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

func TestLossGECommand_Run_DryRun(t *testing.T) {
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

	mockClient.On("NetemContainer", mock.Anything, target, "eth0",
		[]string{"loss", "gemodel", "30.00", "70.00", "50.00", "10.00"},
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "tc-image", false, true).
		Return(nil)

	mockClient.On("StopNetemContainer", mock.Anything, target, "eth0",
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		"tc-image", false, true).
		Return(nil)

	cmd, err := NewLossGECommand(mockClient, gparams, nparams, 30.0, 70.0, 50.0, 10.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}
