package container

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestListNContainersAll_PassesAllFlagToDocker(t *testing.T) {
	mockClient := new(MockClient)
	expected := CreateTestContainers(2)

	mockClient.On("ListContainers", mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.MatchedBy(func(opts ListOpts) bool {
		return opts.All == true
	})).Return(expected, nil)

	containers, err := ListNContainersAll(context.TODO(), mockClient, []string{"c0", "c1"}, "", nil, 0, true)

	assert.NoError(t, err)
	assert.Len(t, containers, 2)
	mockClient.AssertExpectations(t)
}

func TestListNContainersAll_DefaultDoesNotPassAllFlag(t *testing.T) {
	mockClient := new(MockClient)
	expected := CreateTestContainers(2)

	mockClient.On("ListContainers", mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.MatchedBy(func(opts ListOpts) bool {
		return opts.All == false
	})).Return(expected, nil)

	containers, err := ListNContainers(context.TODO(), mockClient, []string{"c0", "c1"}, "", nil, 0)

	assert.NoError(t, err)
	assert.Len(t, containers, 2)
	mockClient.AssertExpectations(t)
}

func TestListNContainersAll_ReturnsError(t *testing.T) {
	mockClient := new(MockClient)

	mockClient.On("ListContainers", mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).Return(([]*Container)(nil), errors.New("docker error"))

	containers, err := ListNContainersAll(context.TODO(), mockClient, []string{"c0"}, "", nil, 0, true)

	assert.Error(t, err)
	assert.Nil(t, containers)
	mockClient.AssertExpectations(t)
}

func TestListNContainersAll_RespectsLimit(t *testing.T) {
	mockClient := new(MockClient)
	expected := CreateTestContainers(5)

	mockClient.On("ListContainers", mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).Return(expected, nil)

	containers, err := ListNContainersAll(context.TODO(), mockClient, []string{"c0", "c1", "c2", "c3", "c4"}, "", nil, 2, true)

	assert.NoError(t, err)
	assert.Len(t, containers, 2)
	mockClient.AssertExpectations(t)
}

func TestRandomContainer(t *testing.T) {
	tests := []struct {
		name       string
		containers []*Container
		expectNil  bool
	}{
		{
			name:       "nil slice returns nil",
			containers: nil,
			expectNil:  true,
		},
		{
			name:       "empty slice returns nil",
			containers: []*Container{},
			expectNil:  true,
		},
		{
			name: "single container returns that container",
			containers: []*Container{
				{ContainerID: "only-one", Labels: map[string]string{}, Networks: map[string]NetworkLink{}},
			},
		},
		{
			name:       "multiple containers returns one of them",
			containers: CreateTestContainers(5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RandomContainer(tt.containers)
			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Contains(t, tt.containers, result)
			}
		})
	}
}

func TestListNContainersAll_LimitEqualToCount(t *testing.T) {
	mockClient := new(MockClient)
	expected := CreateTestContainers(3)

	mockClient.On("ListContainers", mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).
		Return(expected, nil)

	containers, err := ListNContainersAll(context.TODO(), mockClient, []string{"c0", "c1", "c2"}, "", nil, 3, false)

	assert.NoError(t, err)
	assert.Len(t, containers, 3)
	mockClient.AssertExpectations(t)
}

func TestListNContainers_WithLabels(t *testing.T) {
	mockClient := new(MockClient)
	expected := CreateTestContainers(1)
	labels := []string{"env=prod", "tier=web"}

	mockClient.On("ListContainers", mock.Anything, mock.AnythingOfType("container.FilterFunc"),
		mock.MatchedBy(func(opts ListOpts) bool {
			return opts.All == false && len(opts.Labels) == 2 && opts.Labels[0] == "env=prod" && opts.Labels[1] == "tier=web"
		})).Return(expected, nil)

	containers, err := ListNContainers(context.TODO(), mockClient, []string{"c0"}, "", labels, 0)

	assert.NoError(t, err)
	assert.Len(t, containers, 1)
	mockClient.AssertExpectations(t)
}
