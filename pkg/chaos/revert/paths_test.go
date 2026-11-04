package revert

import (
	"context"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/require"
)

func TestNewCommandRejectsEachUnsupportedSchedulingOption(t *testing.T) {
	for _, tc := range []struct {
		name string
		gp   chaos.GlobalParams
	}{
		{name: "interval", gp: chaos.GlobalParams{Interval: 1}},
		{name: "random", gp: chaos.GlobalParams{Random: true}},
		{name: "skip errors", gp: chaos.GlobalParams{SkipErrors: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := container.NewMockClient(t)
			_, err := NewCommand(client, &tc.gp, Params{RunFilter: "old"})
			require.ErrorContains(t, err, "rejects --interval, --random and --skip-error")
		})
	}
}

func TestRecoveryInventoryRejectsNegativeCapAndCanceledContextBeforeListing(t *testing.T) {
	for _, tc := range []struct {
		name string
		gp   *chaos.GlobalParams
		ctx  func() context.Context
		want string
	}{
		{name: "negative target cap", gp: &chaos.GlobalParams{MaxTargets: -1}, ctx: context.Background, want: chaos.ErrNegativeMaxTargets.Error()},
		{name: "canceled before inventory", gp: &chaos.GlobalParams{}, ctx: func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }, want: "context canceled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := container.NewMockClient(t)
			cmd, err := NewCommand(client, tc.gp, Params{RunFilter: "old"})
			require.NoError(t, err)
			err = cmd.Run(tc.ctx(), false)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
