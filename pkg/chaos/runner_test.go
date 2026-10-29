package chaos_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func makeContainers(ids ...string) []*container.Container {
	cs := make([]*container.Container, 0, len(ids))
	for _, id := range ids {
		cs = append(cs, &container.Container{ContainerID: id, ContainerName: id})
	}
	return cs
}

func TestRunOnContainers_EmptyList(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gp := &chaos.GlobalParams{Names: []string{"none"}}

	mockClient.EXPECT().
		ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).
		Return(nil, nil)

	called := false
	err := chaos.RunOnContainers(context.Background(), mockClient, gp, 0, false, true,
		func(_ context.Context, _ *container.Container) error {
			called = true
			return nil
		})
	assert.NoError(t, err)
	assert.False(t, called, "fn must not run when no containers match")
}

func TestRunOnContainers_ListError(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gp := &chaos.GlobalParams{Names: []string{"x"}}

	listErr := errors.New("boom")
	mockClient.EXPECT().
		ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).
		Return(nil, listErr)

	err := chaos.RunOnContainers(context.Background(), mockClient, gp, 0, false, true,
		func(_ context.Context, _ *container.Container) error {
			t.Fatal("fn must not run when list fails")
			return nil
		})
	require.Error(t, err)
	assert.ErrorIs(t, err, listErr)
}

func TestRunOnContainers_SerialSingle(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gp := &chaos.GlobalParams{Names: []string{"a"}}
	cs := makeContainers("a")

	mockClient.EXPECT().
		ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).
		Return(cs, nil)

	var seen []string
	err := chaos.RunOnContainers(context.Background(), mockClient, gp, 0, false, false,
		func(_ context.Context, c *container.Container) error {
			seen = append(seen, c.ContainerID)
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, seen)
}

func TestRunOnContainers_SerialMultiple(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gp := &chaos.GlobalParams{Names: []string{"a", "b", "c"}}
	cs := makeContainers("a", "b", "c")

	mockClient.EXPECT().
		ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).
		Return(cs, nil)

	var seen []string
	err := chaos.RunOnContainers(context.Background(), mockClient, gp, 0, false, false,
		func(_ context.Context, c *container.Container) error {
			seen = append(seen, c.ContainerID)
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, seen, "serial preserves order")
}

func TestRunOnContainers_ParallelMultiple(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gp := &chaos.GlobalParams{Names: []string{"a", "b", "c"}}
	cs := makeContainers("a", "b", "c")

	mockClient.EXPECT().
		ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).
		Return(cs, nil)

	var counter atomic.Int32
	err := chaos.RunOnContainers(context.Background(), mockClient, gp, 0, false, true,
		func(_ context.Context, _ *container.Container) error {
			counter.Add(1)
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, int32(3), counter.Load(), "every container processed once")
}

func TestRunOnContainers_RandomPicksOne(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gp := &chaos.GlobalParams{Names: []string{"a", "b", "c"}}
	cs := makeContainers("a", "b", "c")

	mockClient.EXPECT().
		ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).
		Return(cs, nil)

	var counter atomic.Int32
	err := chaos.RunOnContainers(context.Background(), mockClient, gp, 0, true, true,
		func(_ context.Context, _ *container.Container) error {
			counter.Add(1)
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, int32(1), counter.Load(), "random reduces fan-out to a single container")
}

func TestRunOnContainers_ParallelErrorReturned(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gp := &chaos.GlobalParams{Names: []string{"a", "b"}}
	cs := makeContainers("a", "b")

	mockClient.EXPECT().
		ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).
		Return(cs, nil)

	wantErr := errors.New("fail-b")
	var counter atomic.Int32
	err := chaos.RunOnContainers(context.Background(), mockClient, gp, 0, false, true,
		func(_ context.Context, c *container.Container) error {
			counter.Add(1)
			if c.ContainerID == "b" {
				return wantErr
			}
			return nil
		})
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
	assert.Equal(t, int32(2), counter.Load(), "bare errgroup must let every closure complete; not WithContext")
}

func TestRunOnContainers_SerialStopsOnFirstError(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gp := &chaos.GlobalParams{Names: []string{"a", "b", "c"}}
	cs := makeContainers("a", "b", "c")

	mockClient.EXPECT().
		ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).
		Return(cs, nil)

	wantErr := errors.New("fail-a")
	var seen []string
	err := chaos.RunOnContainers(context.Background(), mockClient, gp, 0, false, false,
		func(_ context.Context, c *container.Container) error {
			seen = append(seen, c.ContainerID)
			if c.ContainerID == "a" {
				return wantErr
			}
			return nil
		})
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
	assert.Equal(t, []string{"a"}, seen, "serial must stop after first failure")
}

func TestRunOnContainers_PassesLabelsAndPatternToLister(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gp := &chaos.GlobalParams{
		Names:   nil,
		Pattern: "web-.*",
		Labels:  []string{"role=api"},
	}
	cs := makeContainers("web-1")

	mockClient.EXPECT().
		ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"),
			container.ListOpts{All: false, Labels: []string{"role=api"}}).
		Return(cs, nil)

	err := chaos.RunOnContainers(context.Background(), mockClient, gp, 0, false, true,
		func(_ context.Context, _ *container.Container) error { return nil })
	require.NoError(t, err)
}

func TestRunOnContainersAll_IncludesStoppedContainers(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gp := &chaos.GlobalParams{Names: []string{"a", "b"}}
	cs := makeContainers("a", "b")

	mockClient.EXPECT().
		ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"),
			container.ListOpts{All: true}).
		Return(cs, nil)

	var seen []string
	err := chaos.RunOnContainersAll(context.Background(), mockClient, gp, 0, false, false,
		func(_ context.Context, c *container.Container) error {
			seen = append(seen, c.ContainerID)
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, seen)
}

func TestRunOnContainersAll_EmptyList(t *testing.T) {
	mockClient := container.NewMockClient(t)
	gp := &chaos.GlobalParams{Names: []string{"none"}}

	mockClient.EXPECT().
		ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"),
			container.ListOpts{All: true}).
		Return(nil, nil)

	called := false
	err := chaos.RunOnContainersAll(context.Background(), mockClient, gp, 0, false, false,
		func(_ context.Context, _ *container.Container) error {
			called = true
			return nil
		})
	require.NoError(t, err)
	assert.False(t, called, "fn must not run when no containers match")
}

func TestRunOnContainers_MaxTargets(t *testing.T) {
	for _, all := range []bool{false, true} {
		for _, parallel := range []bool{false, true} {
			for _, tt := range []struct {
				name           string
				cap, limit     int
				random, dryRun bool
				want           int32
				wantErr        bool
			}{
				{name: "unset", want: 3},
				{name: "under", cap: 4, want: 3},
				{name: "equal", cap: 3, want: 3},
				{name: "over", cap: 2, wantErr: true},
				{name: "dry run over", cap: 2, dryRun: true, wantErr: true},
				{name: "random", cap: 1, random: true, want: 1},
				{name: "limit", cap: 2, limit: 2, want: 2},
				{name: "limit over", cap: 1, limit: 2, wantErr: true},
				{name: "negative", cap: -1, wantErr: true},
			} {
				t.Run(fmt.Sprintf("%s/all=%t/parallel=%t", tt.name, all, parallel), func(t *testing.T) {
					client := container.NewMockClient(t)
					if tt.cap >= 0 {
						client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{All: all}).Return(makeContainers("a", "b", "c"), nil).Once()
					}
					gp := &chaos.GlobalParams{MaxTargets: tt.cap, DryRun: tt.dryRun}
					var calls atomic.Int32
					run := chaos.RunOnContainers
					if all {
						run = chaos.RunOnContainersAll
					}
					err := run(context.Background(), client, gp, tt.limit, tt.random, parallel, func(context.Context, *container.Container) error { calls.Add(1); return nil })
					if tt.wantErr {
						require.Error(t, err)
						if tt.cap < 0 {
							assert.Contains(t, err.Error(), "--max-targets must be non-negative")
						} else {
							count := 3
							if tt.limit > 0 {
								count = tt.limit
							}
							assert.Contains(t, err.Error(), fmt.Sprintf("%d containers selected, --max-targets is %d", count, tt.cap))
						}
					} else {
						require.NoError(t, err)
					}
					assert.Equal(t, tt.want, calls.Load())
				})
			}
		}
	}
}
