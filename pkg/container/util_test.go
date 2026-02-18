package container

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestApplyContainerFilter_SkipsPumba(t *testing.T) {
	labels := map[string]string{"com.gaiaadm.pumba": "true"}
	c := &Container{
		ContainerName: "pumba-container",
		Labels:        labels,
		Networks:      map[string]NetworkLink{},
	}
	flt := filter{Names: []string{"pumba-container"}, Opts: ListOpts{All: false}}
	fn := applyContainerFilter(flt)

	assert.False(t, fn(c))
}

func TestApplyContainerFilter_SkipsPumbaSkip(t *testing.T) {
	labels := map[string]string{"com.gaiaadm.pumba.skip": "true"}
	c := &Container{
		ContainerName: "skip-container",
		Labels:        labels,
		Networks:      map[string]NetworkLink{},
	}
	flt := filter{Names: []string{"skip-container"}, Opts: ListOpts{All: false}}
	fn := applyContainerFilter(flt)

	assert.False(t, fn(c))
}

func TestApplyContainerFilter_MatchesByName(t *testing.T) {
	c := &Container{
		ContainerName: "target",
		Labels:        map[string]string{},
		Networks:      map[string]NetworkLink{},
	}

	flt := filter{Names: []string{"target"}, Opts: ListOpts{All: false}}
	fn := applyContainerFilter(flt)

	assert.True(t, fn(c))
}

func TestApplyContainerFilter_MatchesByNameWithAllFlag(t *testing.T) {
	target := &Container{
		ContainerName: "target",
		Labels:        map[string]string{},
		Networks:      map[string]NetworkLink{},
	}
	other := &Container{
		ContainerName: "other",
		Labels:        map[string]string{},
		Networks:      map[string]NetworkLink{},
	}

	flt := filter{Names: []string{"target"}, Opts: ListOpts{All: true}}
	fn := applyContainerFilter(flt)

	assert.True(t, fn(target))
	assert.False(t, fn(other))
}

func TestApplyContainerFilter_MatchesByPatternWithAllFlag(t *testing.T) {
	matching := &Container{
		ContainerName: "app-web-1",
		Labels:        map[string]string{},
		Networks:      map[string]NetworkLink{},
	}
	nonMatching := &Container{
		ContainerName: "db-postgres-1",
		Labels:        map[string]string{},
		Networks:      map[string]NetworkLink{},
	}

	flt := filter{Pattern: "^app-", Opts: ListOpts{All: true}}
	fn := applyContainerFilter(flt)

	assert.True(t, fn(matching))
	assert.False(t, fn(nonMatching))
}

func TestApplyContainerFilter_MatchesByPattern(t *testing.T) {
	c := &Container{
		ContainerName: "app-web",
		Labels:        map[string]string{},
		Networks:      map[string]NetworkLink{},
	}

	flt := filter{Pattern: "^app-", Opts: ListOpts{All: false}}
	fn := applyContainerFilter(flt)

	assert.True(t, fn(c))
}

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

func TestMatchNames(t *testing.T) {
	tests := []struct {
		name          string
		names         []string
		containerName string
		expected      bool
	}{
		{
			name:          "empty names list",
			names:         []string{},
			containerName: "container1",
			expected:      false,
		},
		{
			name:          "name in the list",
			names:         []string{"container1", "container2"},
			containerName: "container1",
			expected:      true,
		},
		{
			name:          "name not in the list",
			names:         []string{"container1", "container2"},
			containerName: "container3",
			expected:      false,
		},
		{
			name:          "name in the list with leading slash",
			names:         []string{"container1", "container2"},
			containerName: "/container1",
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchNames(tt.names, tt.containerName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		containerName string
		expected      bool
	}{
		{
			name:          "exact match",
			pattern:       "container1",
			containerName: "container1",
			expected:      true,
		},
		{
			name:          "regex match",
			pattern:       "container[0-9]",
			containerName: "container1",
			expected:      true,
		},
		{
			name:          "no match",
			pattern:       "container[0-9]",
			containerName: "containerX",
			expected:      false,
		},
		{
			name:          "match with leading slash",
			pattern:       "container[0-9]",
			containerName: "/container1",
			expected:      true,
		},
		{
			name:          "invalid regex pattern",
			pattern:       "container[",
			containerName: "container1",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchPattern(tt.pattern, tt.containerName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
