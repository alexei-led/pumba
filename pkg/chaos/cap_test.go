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

func TestCommandsKeepGlobalParams(t *testing.T) {
	for _, name := range []string{"kill", "stop", "pause", "restart", "exec", "rm", "stress"} {
		t.Run(name, func(t *testing.T) {
			client := container.NewMockClient(t)
			sink := &recordSink{}
			gp := &chaos.GlobalParams{Names: []string{"a"}, DryRun: true, RunID: "r1", Action: name, Events: sink}
			target := makeContainers("a")[0]
			client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{All: name == "rm"}).Return([]*container.Container{target}, nil).Once()
			var command chaos.Command
			switch name {
			case "kill":
				client.EXPECT().KillContainer(mock.Anything, target, lifecycle.DefaultKillSignal, true).Return(nil).Once()
				var err error
				command, err = lifecycle.NewKillCommand(client, gp, "", 0)
				require.NoError(t, err)
			case "stop":
				client.EXPECT().StopContainer(mock.Anything, target, lifecycle.DeafultWaitTime, true).Return(nil).Once()
				client.EXPECT().StartContainer(mock.Anything, target, true).Return(nil).Once()
				command = lifecycle.NewStopCommand(client, gp, true, time.Hour, 0, 0)
			case "pause":
				client.EXPECT().PauseContainer(mock.Anything, target, true).Return(nil).Once()
				client.EXPECT().UnpauseContainer(mock.Anything, target, true).Return(nil).Once()
				command = lifecycle.NewPauseCommand(client, gp, time.Hour, 0)
			case "restart":
				client.EXPECT().RestartContainer(mock.Anything, target, time.Second, true).Return(nil).Once()
				command = lifecycle.NewRestartCommand(client, gp, time.Second, 0)
			case "exec":
				client.EXPECT().ExecContainer(mock.Anything, target, "true", []string(nil), true).Return(nil).Once()
				command = lifecycle.NewExecCommand(client, gp, "true", nil, 0)
			case "rm":
				client.EXPECT().RemoveContainer(mock.Anything, target, container.RemoveOpts{DryRun: true}).Return(nil).Once()
				command = lifecycle.NewRemoveCommand(client, gp, false, false, false, 0)
			case "stress":
				client.EXPECT().StressContainer(mock.Anything, mock.AnythingOfType("*container.StressRequest")).Return(&container.StressResult{}, nil).Once()
				command = stress.NewStressCommand(client, gp, "image", false, "--cpu 1", time.Hour, 0, false)
			}
			start := time.Now()
			require.NoError(t, command.Run(context.Background(), false))
			assert.Less(t, time.Since(start), time.Second, "dry-run returns without waiting for --duration")
			results := sink.ofType(chaos.EventTargetResult)
			require.Len(t, results, 1, "events reach the sink of the GlobalParams the command received")
			assert.Equal(t, "r1", results[0].RunID)
			assert.Equal(t, chaos.TargetPlanned, results[0].Result.Status)
			assert.Empty(t, sink.ofType(chaos.EventTargetApplied), "dry-run applies nothing")
		})
	}
}
