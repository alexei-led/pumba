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

func TestNewLossStateCommand_Validation(t *testing.T) {
	mockClient := new(container.MockClient)
	gparams := &chaos.GlobalParams{Names: []string{"test"}}
	nparams := &Params{Iface: "eth0", Duration: time.Second}

	tests := []struct {
		name    string
		p13     float64
		p31     float64
		p32     float64
		p23     float64
		p14     float64
		wantErr string
	}{
		{
			name: "valid params",
			p13:  50.0,
			p31:  50.0,
			p32:  50.0,
			p23:  50.0,
			p14:  50.0,
		},
		{
			name:    "negative p13 rejected",
			p13:     -1.0,
			p31:     50.0,
			p32:     50.0,
			p23:     50.0,
			p14:     50.0,
			wantErr: "invalid p13",
		},
		{
			name:    "p13 over 100 rejected",
			p13:     101.0,
			p31:     50.0,
			p32:     50.0,
			p23:     50.0,
			p14:     50.0,
			wantErr: "invalid p13",
		},
		{
			name:    "negative p31 rejected",
			p13:     50.0,
			p31:     -1.0,
			p32:     50.0,
			p23:     50.0,
			p14:     50.0,
			wantErr: "invalid p31",
		},
		{
			name:    "p31 over 100 rejected",
			p13:     50.0,
			p31:     101.0,
			p32:     50.0,
			p23:     50.0,
			p14:     50.0,
			wantErr: "invalid p31",
		},
		{
			name:    "negative p32 rejected",
			p13:     50.0,
			p31:     50.0,
			p32:     -1.0,
			p23:     50.0,
			p14:     50.0,
			wantErr: "invalid p32",
		},
		{
			name:    "p32 over 100 rejected",
			p13:     50.0,
			p31:     50.0,
			p32:     101.0,
			p23:     50.0,
			p14:     50.0,
			wantErr: "invalid p32",
		},
		{
			name:    "negative p23 rejected",
			p13:     50.0,
			p31:     50.0,
			p32:     50.0,
			p23:     -1.0,
			p14:     50.0,
			wantErr: "invalid p23",
		},
		{
			name:    "p23 over 100 rejected",
			p13:     50.0,
			p31:     50.0,
			p32:     50.0,
			p23:     101.0,
			p14:     50.0,
			wantErr: "invalid p23",
		},
		{
			name:    "negative p14 rejected",
			p13:     50.0,
			p31:     50.0,
			p32:     50.0,
			p23:     50.0,
			p14:     -1.0,
			wantErr: "invalid p14",
		},
		{
			name:    "p14 over 100 rejected",
			p13:     50.0,
			p31:     50.0,
			p32:     50.0,
			p23:     50.0,
			p14:     101.0,
			wantErr: "invalid p14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewLossStateCommand(mockClient, gparams, nparams,
				tt.p13, tt.p31, tt.p32, tt.p23, tt.p14)
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

func TestLossStateCommand_Run_DryRun(t *testing.T) {
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
		[]string{"loss", "state", "20.00", "80.00", "30.00", "40.00", "10.00"},
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "tc-image", false, true).
		Return(nil)

	mockClient.On("StopNetemContainer", mock.Anything, target, "eth0",
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		"tc-image", false, true).
		Return(nil)

	cmd, err := NewLossStateCommand(mockClient, gparams, nparams, 20.0, 80.0, 30.0, 40.0, 10.0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}
