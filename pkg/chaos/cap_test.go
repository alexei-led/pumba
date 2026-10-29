package chaos_test

import (
	"context"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/lifecycle"
	"github.com/alexei-led/pumba/pkg/chaos/stress"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCommandsPreserveMaxTargets(t *testing.T) {
	for _, name := range []string{"kill", "stop", "pause", "restart", "exec", "rm", "stress"} {
		t.Run(name, func(t *testing.T) {
			client := container.NewMockClient(t)
			gp := &chaos.GlobalParams{MaxTargets: 1, Names: []string{"a", "b"}}
			client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{All: name == "rm"}).Return(makeContainers("a", "b"), nil).Once()
			var command chaos.Command
			switch name {
			case "kill":
				var err error
				command, err = lifecycle.NewKillCommand(client, gp, "", 0)
				require.NoError(t, err)
			case "stop":
				command = lifecycle.NewStopCommand(client, gp, false, time.Second, 0, 0)
			case "pause":
				command = lifecycle.NewPauseCommand(client, gp, time.Second, 0)
			case "restart":
				command = lifecycle.NewRestartCommand(client, gp, time.Second, 0)
			case "exec":
				command = lifecycle.NewExecCommand(client, gp, "true", nil, 0)
			case "rm":
				command = lifecycle.NewRemoveCommand(client, gp, false, false, false, 0)
			case "stress":
				command = stress.NewStressCommand(client, gp, "image", false, "--cpu 1", time.Second, 0, false)
			}
			assert.ErrorContains(t, command.Run(context.Background(), false), "2 containers selected, --max-targets is 1")
		})
	}
}

func TestMaxTargetsRecheckedOnIntervalAndNotSkipped(t *testing.T) {
	client := container.NewMockClient(t)
	target := makeContainers("a")[0]
	gp := &chaos.GlobalParams{MaxTargets: 1, Interval: time.Nanosecond, DryRun: true, SkipErrors: true}
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).Return([]*container.Container{target}, nil).Once()
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).Return(makeContainers("a", "b"), nil).Once()
	client.EXPECT().KillContainer(mock.Anything, target, lifecycle.DefaultKillSignal, true).Return(nil).Once()
	command, err := lifecycle.NewKillCommand(client, gp, "", 0)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.ErrorContains(t, chaos.RunChaosCommand(ctx, command, gp), "2 containers selected, --max-targets is 1")
}
