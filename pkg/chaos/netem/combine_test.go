package netem

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewCombinedCommandValidation(t *testing.T) {
	tests := []struct {
		name    string
		spec    CombinedSpec
		wantErr string
	}{
		{name: "no effects", wantErr: "at least two"},
		{name: "one effect", spec: CombinedSpec{Delay: &DelayEffect{Time: 100}}, wantErr: "at least two"},
		{
			name:    "invalid delay",
			spec:    CombinedSpec{Delay: &DelayEffect{Time: -1}, Loss: &LossEffect{Percent: 10}},
			wantErr: "non-positive delay",
		},
		{
			name:    "invalid loss",
			spec:    CombinedSpec{Delay: &DelayEffect{Time: 100}, Loss: &LossEffect{Percent: 101}},
			wantErr: "invalid loss percent",
		},
		{
			name:    "invalid corrupt",
			spec:    CombinedSpec{Delay: &DelayEffect{Time: 100}, Corrupt: &CorruptEffect{Percent: -1}},
			wantErr: "invalid corrupt percent",
		},
		{
			name:    "invalid duplicate",
			spec:    CombinedSpec{Delay: &DelayEffect{Time: 100}, Duplicate: &DuplicateEffect{Correlation: 101}},
			wantErr: "invalid duplicate correlation",
		},
		{
			name:    "invalid rate",
			spec:    CombinedSpec{Delay: &DelayEffect{Time: 100}, Rate: &RateEffect{Rate: "invalid"}},
			wantErr: "invalid rate",
		},
		{
			name: "valid effects",
			spec: CombinedSpec{Delay: &DelayEffect{Time: 100}, Loss: &LossEffect{Percent: 20}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewCombinedCommand(nil, &chaos.GlobalParams{}, &container.NetemRequest{}, 0, tt.spec)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, cmd)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, cmd)
		})
	}
}

func TestCombinedCommandRunBuildsOneNetemCommand(t *testing.T) {
	client := container.NewMockClient(t)
	target := &container.Container{ContainerID: "target-id", ContainerName: "target"}
	gp := &chaos.GlobalParams{Names: []string{"target"}, DryRun: true}
	req := &container.NetemRequest{Interface: "eth0", Duration: time.Millisecond, DryRun: true}
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{All: false}).
		Return([]*container.Container{target}, nil)
	expected := &container.NetemRequest{
		Container: target,
		Interface: "eth0",
		Command: []string{
			"delay", "200ms", "50ms", "25.50", "distribution", "normal",
			"loss", "10.00", "5.00",
			"duplicate", "3.00",
			"corrupt", "1.00",
			"rate", "100kbit", "0", "53",
		},
		Duration: time.Millisecond,
		DryRun:   true,
	}
	client.EXPECT().NetemContainer(mock.Anything, expected).Return(nil)
	client.EXPECT().StopNetemContainer(mock.Anything, expected).Return(nil)

	cmd, err := NewCombinedCommand(client, gp, req, 0, CombinedSpec{
		Delay:     &DelayEffect{Time: 200, Jitter: 50, Correlation: 25.5, Distribution: "normal"},
		Loss:      &LossEffect{Percent: 10, Correlation: 5},
		Duplicate: &DuplicateEffect{Percent: 3},
		Corrupt:   &CorruptEffect{Percent: 1},
		Rate:      &RateEffect{Rate: "100kbit", CellSize: 53},
	})
	require.NoError(t, err)

	require.NoError(t, cmd.Run(context.Background(), false))
}

func TestCombinedCommandRunResolvesNamedTrafficTarget(t *testing.T) {
	client := container.NewMockClient(t)
	peer := &container.Container{
		ContainerID: "peer-id", ContainerName: "peer",
		Networks: map[string]container.NetworkLink{"bridge": {IPv4Address: "10.0.0.9"}},
	}
	target := &container.Container{ContainerID: "target-id", ContainerName: "target"}
	gp := &chaos.GlobalParams{Names: []string{"target"}, DryRun: true}
	req := &container.NetemRequest{TargetNames: []string{"peer"}, Duration: time.Millisecond, DryRun: true}
	client.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{peer, target}, nil).Once()
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{All: false}).
		Return([]*container.Container{target}, nil).Once()
	combinedRequest := mock.MatchedBy(func(actual *container.NetemRequest) bool {
		return len(actual.IPs) == 1 && actual.IPs[0].String() == "10.0.0.9/32" && len(actual.TargetNames) == 0
	})
	client.EXPECT().NetemContainer(mock.Anything, combinedRequest).Return(nil)
	client.EXPECT().StopNetemContainer(mock.Anything, combinedRequest).Return(nil)
	cmd, err := NewCombinedCommand(client, gp, req, 0, CombinedSpec{
		Delay: &DelayEffect{Time: 100},
		Loss:  &LossEffect{Percent: 10},
	})
	require.NoError(t, err)

	require.NoError(t, cmd.Run(context.Background(), false))
}

func TestCombinedCommandRunWithRandom(t *testing.T) {
	client := container.NewMockClient(t)
	first := &container.Container{ContainerID: "first-id", ContainerName: "first"}
	second := &container.Container{ContainerID: "second-id", ContainerName: "second"}
	gp := &chaos.GlobalParams{Names: []string{"first", "second"}, DryRun: true}
	req := &container.NetemRequest{Duration: time.Millisecond, DryRun: true}
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{All: false}).
		Return([]*container.Container{first, second}, nil)
	combinedRequest := mock.MatchedBy(func(actual *container.NetemRequest) bool {
		return (actual.Container == first || actual.Container == second) &&
			slices.Equal([]string{"delay", "100ms", "loss", "10.00"}, actual.Command)
	})
	client.EXPECT().NetemContainer(mock.Anything, combinedRequest).Return(nil).Once()
	client.EXPECT().StopNetemContainer(mock.Anything, combinedRequest).Return(nil).Once()
	cmd, err := NewCombinedCommand(client, gp, req, 0, CombinedSpec{
		Delay: &DelayEffect{Time: 100},
		Loss:  &LossEffect{Percent: 10},
	})
	require.NoError(t, err)

	require.NoError(t, cmd.Run(context.Background(), true))
}

func TestCombinedCommandRunNoContainers(t *testing.T) {
	client := container.NewMockClient(t)
	gp := &chaos.GlobalParams{Names: []string{"missing"}}
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{All: false}).
		Return(nil, nil)
	cmd, err := NewCombinedCommand(client, gp, &container.NetemRequest{}, 0, CombinedSpec{
		Delay: &DelayEffect{Time: 100},
		Loss:  &LossEffect{Percent: 10},
	})
	require.NoError(t, err)

	require.NoError(t, cmd.Run(context.Background(), false))
}
