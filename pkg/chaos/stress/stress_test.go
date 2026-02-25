package stress

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testContainer(id, name string) *container.Container {
	return &container.Container{
		ContainerID:   id,
		ContainerName: name,
		Labels:        map[string]string{},
		Networks:      map[string]container.NetworkLink{},
	}
}

func TestStressCommand_Run(t *testing.T) {
	const (
		image      = "stress-ng:latest"
		stressArgs = "--cpu 2 --timeout 30s"
		targetID   = "abc123"
		targetName = "target"
		stressID   = "stress-container-id"
	)
	duration := 500 * time.Millisecond
	target := testContainer(targetID, targetName)
	stressors := []string{"--cpu", "2", "--timeout", "30s"}
	anyFilter := mock.AnythingOfType("container.FilterFunc")
	listOpts := container.ListOpts{All: false, Labels: nil}

	tests := []struct {
		name      string
		params    *chaos.GlobalParams
		random    bool
		setupMock func(*container.MockClient)
		wantErr   string
	}{
		{
			name:   "no matching containers returns nil",
			params: &chaos.GlobalParams{Names: []string{targetName}},
			setupMock: func(m *container.MockClient) {
				m.EXPECT().ListContainers(mock.Anything, anyFilter, listOpts).
					Return([]*container.Container{}, nil)
			},
		},
		{
			name:   "list containers error propagates",
			params: &chaos.GlobalParams{Names: []string{targetName}},
			setupMock: func(m *container.MockClient) {
				m.EXPECT().ListContainers(mock.Anything, anyFilter, listOpts).
					Return(nil, errors.New("docker daemon unavailable"))
			},
			wantErr: "docker daemon unavailable",
		},
		{
			name:   "dry run calls StressContainer with dryRun=true",
			params: &chaos.GlobalParams{Names: []string{targetName}, DryRun: true},
			setupMock: func(m *container.MockClient) {
				m.EXPECT().ListContainers(mock.Anything, anyFilter, listOpts).
					Return([]*container.Container{target}, nil)
				m.EXPECT().StressContainer(mock.Anything, target, stressors, image, false, duration, false, true).
					Return("", nil, nil, nil)
			},
		},
		{
			name:   "StressContainer error propagates",
			params: &chaos.GlobalParams{Names: []string{targetName}},
			setupMock: func(m *container.MockClient) {
				m.EXPECT().ListContainers(mock.Anything, anyFilter, listOpts).
					Return([]*container.Container{target}, nil)
				m.EXPECT().StressContainer(mock.Anything, target, stressors, image, false, duration, false, false).
					Return("", nil, nil, errors.New("image pull failed"))
			},
			wantErr: "image pull failed",
		},
		{
			name:   "stress-ng completes via output channel",
			params: &chaos.GlobalParams{Names: []string{targetName}},
			setupMock: func(m *container.MockClient) {
				output := make(chan string, 1)
				outerr := make(chan error, 1)
				output <- "stress-ng: info: completed"
				m.EXPECT().ListContainers(mock.Anything, anyFilter, listOpts).
					Return([]*container.Container{target}, nil)
				m.EXPECT().StressContainer(mock.Anything, target, stressors, image, false, duration, false, false).
					Return(stressID, (<-chan string)(output), (<-chan error)(outerr), nil)
			},
		},
		{
			name:   "stress-ng error channel propagates error",
			params: &chaos.GlobalParams{Names: []string{targetName}},
			setupMock: func(m *container.MockClient) {
				output := make(chan string, 1)
				outerr := make(chan error, 1)
				outerr <- errors.New("stress-ng: error: OOM killed")
				m.EXPECT().ListContainers(mock.Anything, anyFilter, listOpts).
					Return([]*container.Container{target}, nil)
				m.EXPECT().StressContainer(mock.Anything, target, stressors, image, false, duration, false, false).
					Return(stressID, (<-chan string)(output), (<-chan error)(outerr), nil)
			},
			wantErr: "OOM killed",
		},
		{
			name:   "context cancellation stops stress container",
			params: &chaos.GlobalParams{Names: []string{targetName}},
			setupMock: func(m *container.MockClient) {
				output := make(chan string)
				outerr := make(chan error)
				m.EXPECT().ListContainers(mock.Anything, anyFilter, listOpts).
					Return([]*container.Container{target}, nil)
				m.EXPECT().StressContainer(mock.Anything, target, stressors, image, false, duration, false, false).
					Return(stressID, (<-chan string)(output), (<-chan error)(outerr), nil)
				m.EXPECT().StopContainerWithID(mock.Anything, stressID, defaultStopTimeout, false).
					Return(nil)
			},
		},
		{
			name:   "random mode picks one from multiple containers",
			params: &chaos.GlobalParams{Names: []string{targetName}, DryRun: true},
			random: true,
			setupMock: func(m *container.MockClient) {
				c1 := testContainer("id1", "c1")
				c2 := testContainer("id2", "c2")
				m.EXPECT().ListContainers(mock.Anything, anyFilter, listOpts).
					Return([]*container.Container{c1, c2}, nil)
				m.EXPECT().StressContainer(mock.Anything, mock.AnythingOfType("*container.Container"),
					stressors, image, false, duration, false, true).
					Return("", nil, nil, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(container.MockClient)
			tt.setupMock(mockClient)

			cmd := NewStressCommand(mockClient, tt.params, image, false, stressArgs, duration, 0, false)

			ctx := context.Background()
			if tt.name == "context cancellation stops stress container" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				go func() {
					time.Sleep(50 * time.Millisecond)
					cancel()
				}()
			}

			err := cmd.Run(ctx, tt.random)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestStressCommand_Run_InjectCgroup(t *testing.T) {
	target := testContainer("abc123", "target")
	stressors := []string{"--cpu", "4"}
	duration := 500 * time.Millisecond
	image := "ghcr.io/pumba/stress-ng:latest"
	anyFilter := mock.AnythingOfType("container.FilterFunc")
	listOpts := container.ListOpts{All: false, Labels: nil}

	mockClient := new(container.MockClient)
	output := make(chan string, 1)
	outerr := make(chan error, 1)
	output <- "done"

	mockClient.EXPECT().ListContainers(mock.Anything, anyFilter, listOpts).
		Return([]*container.Container{target}, nil)
	mockClient.EXPECT().StressContainer(mock.Anything, target, stressors, image, true, duration, true, false).
		Return("stress-id", (<-chan string)(output), (<-chan error)(outerr), nil)

	params := &chaos.GlobalParams{Names: []string{"target"}}
	cmd := NewStressCommand(mockClient, params, image, true, "--cpu 4", duration, 0, true)
	err := cmd.Run(context.Background(), false)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestStressCommand_Run_DurationTimeout(t *testing.T) {
	target := testContainer("abc123", "target")
	stressors := []string{"--cpu", "2"}
	duration := 100 * time.Millisecond
	image := "stress-ng:latest"
	anyFilter := mock.AnythingOfType("container.FilterFunc")
	listOpts := container.ListOpts{All: false, Labels: nil}

	mockClient := new(container.MockClient)
	output := make(chan string)
	outerr := make(chan error)

	mockClient.EXPECT().ListContainers(mock.Anything, anyFilter, listOpts).
		Return([]*container.Container{target}, nil)
	mockClient.EXPECT().StressContainer(mock.Anything, target, stressors, image, false, duration, false, false).
		Return("stress-id", (<-chan string)(output), (<-chan error)(outerr), nil)
	mockClient.EXPECT().StopContainerWithID(mock.Anything, "stress-id", defaultStopTimeout, false).
		Return(nil)

	params := &chaos.GlobalParams{Names: []string{"target"}}
	cmd := NewStressCommand(mockClient, params, image, false, "--cpu 2", duration, 0, false)
	err := cmd.Run(context.Background(), false)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestStressCommand_Run_StopContainerError(t *testing.T) {
	target := testContainer("abc123", "target")
	stressors := []string{"--cpu", "2"}
	duration := 100 * time.Millisecond
	image := "stress-ng:latest"
	anyFilter := mock.AnythingOfType("container.FilterFunc")
	listOpts := container.ListOpts{All: false, Labels: nil}

	mockClient := new(container.MockClient)
	output := make(chan string)
	outerr := make(chan error)

	mockClient.EXPECT().ListContainers(mock.Anything, anyFilter, listOpts).
		Return([]*container.Container{target}, nil)
	mockClient.EXPECT().StressContainer(mock.Anything, target, stressors, image, false, duration, false, false).
		Return("stress-id", (<-chan string)(output), (<-chan error)(outerr), nil)
	mockClient.EXPECT().StopContainerWithID(mock.Anything, "stress-id", defaultStopTimeout, false).
		Return(errors.New("container already removed"))

	params := &chaos.GlobalParams{Names: []string{"target"}}
	cmd := NewStressCommand(mockClient, params, image, false, "--cpu 2", duration, 0, false)
	err := cmd.Run(context.Background(), false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "container already removed")
	mockClient.AssertExpectations(t)
}
