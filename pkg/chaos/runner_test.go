package chaos_test

import (
	"context"
	"errors"
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
	err := chaos.RunOnContainers(context.Background(), mockClient, gp, 0, false, true,
		func(_ context.Context, c *container.Container) error {
			if c.ContainerID == "b" {
				return wantErr
			}
			return nil
		})
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
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
