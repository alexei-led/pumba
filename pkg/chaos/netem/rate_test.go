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

func TestParseRate(t *testing.T) {
	tests := []struct {
		rate    string
		want    string
		wantErr bool
	}{
		{"100mbit", "100mbit", false},
		{"1gbit", "1gbit", false},
		{"10kbit", "10kbit", false},
		{"100", "", true},
		{"bit", "", true},
		{"100kb", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.rate, func(t *testing.T) {
			got, err := parseRate(tt.rate)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}


func TestRateCommand_Run_DryRun(t *testing.T) {
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

	// rate 100mbit, packetOverhead 10, cellSize 20, cellOverhead 30
	// → netem cmd: ["rate", "100mbit", "10", "20", "30"]
	mockClient.On("NetemContainer", mock.Anything, target, "eth0",
		[]string{"rate", "100mbit", "10", "20", "30"},
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		100*time.Millisecond, "tc-image", false, true).
		Return(nil)

	mockClient.On("StopNetemContainer", mock.Anything, target, "eth0",
		([]*net.IPNet)(nil), []string(nil), []string(nil),
		"tc-image", false, true).
		Return(nil)

	cmd, err := NewRateCommand(mockClient, gparams, nparams, "100mbit", 10, 20, 30)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), false)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}
