package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchNames(t *testing.T) {
	tests := []struct {
		name          string
		names         []string
		containerName string
		containerID   string
		expected      bool
	}{
		{
			name:          "empty names list",
			names:         []string{},
			containerName: "container1",
			expected:      false,
		},
		{
			name:          "empty container name",
			names:         []string{"container1"},
			containerName: "",
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
		{
			name:          "matches by container ID",
			names:         []string{"abc123def456"},
			containerName: "/my-container",
			containerID:   "abc123def456",
			expected:      true,
		},
		{
			name:          "no match by name or ID",
			names:         []string{"abc123def456"},
			containerName: "/my-container",
			containerID:   "xyz789",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchNames(tt.names, tt.containerName, tt.containerID)
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
			name:          "empty container name",
			pattern:       "container[0-9]",
			containerName: "",
			expected:      false,
		},
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

func TestApplyContainerFilter(t *testing.T) {
	tests := []struct {
		name      string
		container *Container
		filter    filter
		expected  bool
	}{
		{
			name: "skips pumba-labeled container",
			container: &Container{
				ContainerName: "pumba-container",
				Labels:        map[string]string{"com.gaiaadm.pumba": "true"},
				Networks:      map[string]NetworkLink{},
			},
			filter:   filter{Names: []string{"pumba-container"}, Opts: ListOpts{All: false}},
			expected: false,
		},
		{
			name: "skips pumba-skip-labeled container",
			container: &Container{
				ContainerName: "skip-container",
				Labels:        map[string]string{"com.gaiaadm.pumba.skip": "true"},
				Networks:      map[string]NetworkLink{},
			},
			filter:   filter{Names: []string{"skip-container"}, Opts: ListOpts{All: false}},
			expected: false,
		},
		{
			name: "matches by name",
			container: &Container{
				ContainerName: "target",
				Labels:        map[string]string{},
				Networks:      map[string]NetworkLink{},
			},
			filter:   filter{Names: []string{"target"}, Opts: ListOpts{All: false}},
			expected: true,
		},
		{
			name: "matches named target with all flag",
			container: &Container{
				ContainerName: "target",
				Labels:        map[string]string{},
				Networks:      map[string]NetworkLink{},
			},
			filter:   filter{Names: []string{"target"}, Opts: ListOpts{All: true}},
			expected: true,
		},
		{
			name: "excludes non-target with all flag",
			container: &Container{
				ContainerName: "other",
				Labels:        map[string]string{},
				Networks:      map[string]NetworkLink{},
			},
			filter:   filter{Names: []string{"target"}, Opts: ListOpts{All: true}},
			expected: false,
		},
		{
			name: "matches by pattern with all flag",
			container: &Container{
				ContainerName: "app-web-1",
				Labels:        map[string]string{},
				Networks:      map[string]NetworkLink{},
			},
			filter:   filter{Pattern: "^app-", Opts: ListOpts{All: true}},
			expected: true,
		},
		{
			name: "excludes non-matching pattern with all flag",
			container: &Container{
				ContainerName: "db-postgres-1",
				Labels:        map[string]string{},
				Networks:      map[string]NetworkLink{},
			},
			filter:   filter{Pattern: "^app-", Opts: ListOpts{All: true}},
			expected: false,
		},
		{
			name: "matches by pattern",
			container: &Container{
				ContainerName: "app-web",
				Labels:        map[string]string{},
				Networks:      map[string]NetworkLink{},
			},
			filter:   filter{Pattern: "^app-", Opts: ListOpts{All: false}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := applyContainerFilter(tt.filter)
			assert.Equal(t, tt.expected, fn(tt.container))
		})
	}
}
