package containerd

import (
	"testing"
)

func TestResolveContainerName(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		labels   map[string]string
		expected string
	}{
		{
			name:     "no labels - returns ID",
			id:       "abc123",
			labels:   map[string]string{},
			expected: "abc123",
		},
		{
			name:     "nil labels - returns ID",
			id:       "abc123",
			labels:   nil,
			expected: "abc123",
		},
		{
			name: "kubernetes full labels",
			id:   "abc123",
			labels: map[string]string{
				"io.kubernetes.container.name": "nginx",
				"io.kubernetes.pod.name":       "web-abc123",
				"io.kubernetes.pod.namespace":  "default",
			},
			expected: "default/web-abc123/nginx",
		},
		{
			name: "kubernetes container + pod only",
			id:   "abc123",
			labels: map[string]string{
				"io.kubernetes.container.name": "nginx",
				"io.kubernetes.pod.name":       "web-abc123",
			},
			expected: "web-abc123/nginx",
		},
		{
			name: "kubernetes container name only",
			id:   "abc123",
			labels: map[string]string{
				"io.kubernetes.container.name": "nginx",
			},
			expected: "nginx",
		},
		{
			name: "nerdctl name",
			id:   "abc123",
			labels: map[string]string{
				"nerdctl/name": "my-container",
			},
			expected: "my-container",
		},
		{
			name: "docker compose service",
			id:   "abc123",
			labels: map[string]string{
				"com.docker.compose.service": "redis",
			},
			expected: "redis",
		},
		{
			name: "kubernetes takes priority over nerdctl",
			id:   "abc123",
			labels: map[string]string{
				"io.kubernetes.container.name": "nginx",
				"io.kubernetes.pod.name":       "web-pod",
				"io.kubernetes.pod.namespace":  "prod",
				"nerdctl/name":                 "my-nerdctl",
			},
			expected: "prod/web-pod/nginx",
		},
		{
			name: "nerdctl takes priority over compose",
			id:   "abc123",
			labels: map[string]string{
				"nerdctl/name":                   "my-nerdctl",
				"com.docker.compose.service":     "redis",
			},
			expected: "my-nerdctl",
		},
		{
			name: "unrelated labels - returns ID",
			id:   "abc123",
			labels: map[string]string{
				"app": "web",
				"env": "prod",
			},
			expected: "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveContainerName(tt.id, tt.labels)
			if result != tt.expected {
				t.Errorf("resolveContainerName(%q, %v) = %q, want %q", tt.id, tt.labels, result, tt.expected)
			}
		})
	}
}
