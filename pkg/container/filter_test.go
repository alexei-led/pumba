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
