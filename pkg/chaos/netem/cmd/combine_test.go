package cmd

import (
	"context"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func TestNewCombineCLICommandContract(t *testing.T) {
	runtime, _, calls := fakeRuntime(t)
	cmd := NewCombineCLICommand(context.Background(), runtime)

	assertConstructorContract(t, cmd, "combine")
	assert.Equal(t, 0, *calls)
}

func TestCombineCLIActionParsesContainerAfterSeparator(t *testing.T) {
	runtime, client, calls := fakeRuntime(t)
	cmd := NewCombineCLICommand(context.Background(), runtime)
	parent := netemContext(t, nil)
	ctx := childContext(t, parent, cmd.Flags, []string{
		"--delay", "--delay-time", "100",
		"--loss", "--loss-percent", "10",
		"--", "target",
	})
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{All: false}).
		Return(nil, nil)

	action, ok := cmd.Action.(func(*cli.Context) error)
	require.True(t, ok)
	require.NoError(t, action(ctx))
	assert.Equal(t, 1, *calls)
}

func TestParseCombineParams(t *testing.T) {
	cmd := NewCombineCLICommand(context.Background(), nilRuntime())
	parent := netemContext(t, []string{"--duration", "1s", "--interface", "eth0", "--target", "peer"})
	ctx := childContext(t, parent, cmd.Flags, []string{
		"--delay", "--delay-time", "200", "--delay-jitter", "50",
		"--loss", "--loss-percent", "10",
		"--corrupt", "--corrupt-percent", "2",
		"--duplicate", "--duplicate-percent", "3",
		"--rate", "--rate-value", "1mbit",
		"--", "target",
	})

	params, err := parseCombineParams(cliflags.NewV1(ctx), defaultGlobalParams())

	require.NoError(t, err)
	require.NotNil(t, params.Effects.Delay)
	assert.Equal(t, 200, params.Effects.Delay.Time)
	assert.Equal(t, 50, params.Effects.Delay.Jitter)
	require.NotNil(t, params.Effects.Loss)
	assert.InDelta(t, 10.0, params.Effects.Loss.Percent, 0.001)
	require.NotNil(t, params.Effects.Corrupt)
	require.NotNil(t, params.Effects.Duplicate)
	require.NotNil(t, params.Effects.Rate)
	assert.Equal(t, "1mbit", params.Effects.Rate.Rate)
	assert.Equal(t, []string{"peer"}, params.Base.TargetNames)
	assert.Equal(t, []string{"target"}, []string(ctx.Args()))
}

func TestParseCombineParamsRequiresEffectSwitchForParameters(t *testing.T) {
	cmd := NewCombineCLICommand(context.Background(), nilRuntime())
	parent := netemContext(t, nil)
	ctx := childContext(t, parent, cmd.Flags, []string{
		"--delay-time", "200",
		"--loss", "--loss-percent", "10",
		"target",
	})

	_, err := parseCombineParams(cliflags.NewV1(ctx), defaultGlobalParams())

	require.ErrorContains(t, err, "--delay-time requires --delay")
}

func TestBuildCombineCommandRequiresTwoEffects(t *testing.T) {
	_, err := buildCombineCommand(nil, defaultGlobalParams(), CombineParams{
		Effects: netem.CombinedSpec{Loss: &netem.LossEffect{Percent: 10}},
	})

	require.ErrorContains(t, err, "at least two")
}
