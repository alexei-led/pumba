package container

import (
	"testing"

	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/assert"
)

func TestDockerInspectToContainer(t *testing.T) {
	tests := []struct {
		name     string
		info     ctypes.InspectResponse
		img      *imagetypes.InspectResponse
		expected *Container
	}{
		{
			name: "full running container",
			info: ctypes.InspectResponse{
				ContainerJSONBase: &ctypes.ContainerJSONBase{
					ID:    "abc123",
					Name:  "/mycontainer",
					Image: "nginx:1.25",
					State: &ctypes.State{Running: true},
				},
				Config: &ctypes.Config{
					Labels: map[string]string{"env": "prod"},
				},
				NetworkSettings: &ctypes.NetworkSettings{
					Networks: map[string]*network.EndpointSettings{
						"bridge": {Links: []string{"db:db"}},
					},
				},
			},
			img: &imagetypes.InspectResponse{ID: "sha256:img123"},
			expected: &Container{
				ContainerID:   "abc123",
				ContainerName: "/mycontainer",
				Image:         "nginx:1.25",
				ImageID:       "sha256:img123",
				State:         StateRunning,
				Labels:        map[string]string{"env": "prod"},
				Networks:      map[string]NetworkLink{"bridge": {Links: []string{"db:db"}}},
			},
		},
		{
			name: "exited container",
			info: ctypes.InspectResponse{
				ContainerJSONBase: &ctypes.ContainerJSONBase{
					ID:    "def456",
					Name:  "/stopped",
					Image: "alpine",
					State: &ctypes.State{Running: false},
				},
				Config:          &ctypes.Config{Labels: map[string]string{}},
				NetworkSettings: &ctypes.NetworkSettings{Networks: map[string]*network.EndpointSettings{}},
			},
			img: &imagetypes.InspectResponse{ID: "sha256:alpine123"},
			expected: &Container{
				ContainerID:   "def456",
				ContainerName: "/stopped",
				Image:         "alpine",
				ImageID:       "sha256:alpine123",
				State:         StateExited,
				Labels:        map[string]string{},
				Networks:      map[string]NetworkLink{},
			},
		},
		{
			name: "nil ContainerJSONBase",
			info: ctypes.InspectResponse{
				ContainerJSONBase: nil,
				Config:            nil,
				NetworkSettings:   nil,
			},
			img: &imagetypes.InspectResponse{ID: "sha256:img"},
			expected: &Container{
				ContainerID:   "",
				ContainerName: "",
				Image:         "",
				ImageID:       "sha256:img",
				State:         "",
				Labels:        map[string]string{},
				Networks:      map[string]NetworkLink{},
			},
		},
		{
			name: "nil State in ContainerJSONBase",
			info: ctypes.InspectResponse{
				ContainerJSONBase: &ctypes.ContainerJSONBase{
					ID:    "ghi789",
					Name:  "/nostate",
					Image: "busybox",
					State: nil,
				},
				Config:          &ctypes.Config{Labels: map[string]string{"a": "b"}},
				NetworkSettings: nil,
			},
			img: &imagetypes.InspectResponse{ID: "sha256:bb"},
			expected: &Container{
				ContainerID:   "ghi789",
				ContainerName: "/nostate",
				Image:         "busybox",
				ImageID:       "sha256:bb",
				State:         "",
				Labels:        map[string]string{"a": "b"},
				Networks:      map[string]NetworkLink{},
			},
		},
		{
			name: "multiple networks",
			info: ctypes.InspectResponse{
				ContainerJSONBase: &ctypes.ContainerJSONBase{
					ID:    "multi",
					Name:  "/multi-net",
					Image: "app",
					State: &ctypes.State{Running: true},
				},
				Config: &ctypes.Config{Labels: map[string]string{}},
				NetworkSettings: &ctypes.NetworkSettings{
					Networks: map[string]*network.EndpointSettings{
						"frontend": {Links: []string{"api:api"}},
						"backend":  {Links: []string{"db:db", "cache:cache"}},
					},
				},
			},
			img: &imagetypes.InspectResponse{ID: "sha256:app"},
			expected: &Container{
				ContainerID:   "multi",
				ContainerName: "/multi-net",
				Image:         "app",
				ImageID:       "sha256:app",
				State:         StateRunning,
				Labels:        map[string]string{},
				Networks: map[string]NetworkLink{
					"frontend": {Links: []string{"api:api"}},
					"backend":  {Links: []string{"db:db", "cache:cache"}},
				},
			},
		},
		{
			name: "nil Config labels",
			info: ctypes.InspectResponse{
				ContainerJSONBase: &ctypes.ContainerJSONBase{
					ID: "nolabels", Name: "/nolabels", Image: "x",
					State: &ctypes.State{Running: true},
				},
				Config:          &ctypes.Config{Labels: nil},
				NetworkSettings: nil,
			},
			img: &imagetypes.InspectResponse{ID: "img"},
			expected: &Container{
				ContainerID: "nolabels", ContainerName: "/nolabels",
				Image: "x", ImageID: "img", State: StateRunning,
				Labels: map[string]string{}, Networks: map[string]NetworkLink{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dockerInspectToContainer(tt.info, tt.img)
			assert.Equal(t, tt.expected, result)
		})
	}
}
