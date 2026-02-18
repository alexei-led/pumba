package docker

import (
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
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
		expected *ctr.Container
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
			expected: &ctr.Container{
				ContainerID:   "abc123",
				ContainerName: "/mycontainer",
				Image:         "nginx:1.25",
				ImageID:       "sha256:img123",
				State:         ctr.StateRunning,
				Labels:        map[string]string{"env": "prod"},
				Networks:      map[string]ctr.NetworkLink{"bridge": {Links: []string{"db:db"}}},
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
			expected: &ctr.Container{
				ContainerID:   "def456",
				ContainerName: "/stopped",
				Image:         "alpine",
				ImageID:       "sha256:alpine123",
				State:         ctr.StateExited,
				Labels:        map[string]string{},
				Networks:      map[string]ctr.NetworkLink{},
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
			expected: &ctr.Container{
				ContainerID:   "",
				ContainerName: "",
				Image:         "",
				ImageID:       "sha256:img",
				State:         "",
				Labels:        map[string]string{},
				Networks:      map[string]ctr.NetworkLink{},
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
			expected: &ctr.Container{
				ContainerID:   "ghi789",
				ContainerName: "/nostate",
				Image:         "busybox",
				ImageID:       "sha256:bb",
				State:         "",
				Labels:        map[string]string{"a": "b"},
				Networks:      map[string]ctr.NetworkLink{},
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
			expected: &ctr.Container{
				ContainerID:   "multi",
				ContainerName: "/multi-net",
				Image:         "app",
				ImageID:       "sha256:app",
				State:         ctr.StateRunning,
				Labels:        map[string]string{},
				Networks: map[string]ctr.NetworkLink{
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
			expected: &ctr.Container{
				ContainerID: "nolabels", ContainerName: "/nolabels",
				Image: "x", ImageID: "img", State: ctr.StateRunning,
				Labels: map[string]string{}, Networks: map[string]ctr.NetworkLink{},
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
