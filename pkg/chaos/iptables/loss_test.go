package iptables

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

func TestNewLossCommand_Validation(t *testing.T) {
	mockClient := new(container.MockClient)
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
	mockClient := new(container.MockClient)
	gparams := &chaos.GlobalParams{Names: []string{"nonexistent"}}
	params := &Params{Iface: "eth0", Duration: time.Second}

	mockClient.On("ListContainers", mock.Anything,
		mock.AnythingOfType("container.FilterFunc"),
		container.ListOpts{All: false, Labels: nil}).
		Return([]*container.Container{}, nil)

	cmd, err := NewLossCommand(mockClient, gparams, params, ModeRandom, 0.5, 0, 0)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}
