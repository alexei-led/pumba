package docker

import (
	"context"
	"errors"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestListContainers_Success(t *testing.T) {
	containerDetails := DetailsResponse(AsMap("ID", "foo", "Name", "/bar", "Image", "abc123"))
	allContainersResponse := Containers(Response(AsMap(
		"ID", "foo",
		"Names", []string{"bar"})),
	)
	imageDetails := ImageDetailsResponse(AsMap("ID", "abc123"))

	api := NewMockEngine(t)
	api.EXPECT().ContainerList(mock.Anything, mock.Anything).Return(allContainersResponse, nil)
	api.EXPECT().ContainerInspect(mock.Anything, "foo").Return(containerDetails, nil)
	api.EXPECT().ImageInspect(mock.Anything, "abc123").Return(imageDetails, nil)

	client := dockerClient{containerAPI: api, imageAPI: api}

	// Verify that dockerClient implements container.Client interface
	var _ ctr.Client = (*dockerClient)(nil)

	containers, err := client.ListContainers(context.TODO(), mockAllContainers, ctr.ListOpts{All: true})

	assert.NoError(t, err)
	assert.Len(t, containers, 1)
	assert.Equal(t, "foo", containers[0].ID())
	assert.Equal(t, "/bar", containers[0].Name())
	assert.Equal(t, "abc123", containers[0].ImageID)
	api.AssertExpectations(t)
}

func TestListContainers_Filter(t *testing.T) {
	containerDetails := DetailsResponse(AsMap("ID", "foo", "Name", "/bar", "Image", "abc123"))
	allContainersResponse := Containers(Response(AsMap(
		"ID", "foo",
		"Names", []string{"bar"})),
	)
	imageDetails := ImageDetailsResponse(AsMap("ID", "abc123"))

	api := NewMockEngine(t)
	api.EXPECT().ContainerList(mock.Anything, mock.Anything).Return(allContainersResponse, nil)
	api.EXPECT().ContainerInspect(mock.Anything, "foo").Return(containerDetails, nil)
	api.EXPECT().ImageInspect(mock.Anything, "abc123").Return(imageDetails, nil)

	client := dockerClient{containerAPI: api, imageAPI: api}
	containers, err := client.ListContainers(context.TODO(), mockNoContainers, ctr.ListOpts{})

	assert.NoError(t, err)
	assert.Len(t, containers, 0)
	api.AssertExpectations(t)
}

func TestListContainers_ListError(t *testing.T) {
	api := NewMockEngine(t)
	api.EXPECT().ContainerList(mock.Anything, mock.Anything).Return(Containers(), errors.New("oops"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	_, err := client.ListContainers(context.TODO(), mockAllContainers, ctr.ListOpts{All: true})

	assert.Error(t, err)
	assert.EqualError(t, err, "failed to list containers: oops")
	api.AssertExpectations(t)
}

func TestListContainers_InspectContainerError(t *testing.T) {
	api := NewMockEngine(t)
	allContainersResponse := Containers(Response(AsMap(
		"ID", "foo",
		"Names", []string{"bar"})),
	)
	api.EXPECT().ContainerList(mock.Anything, mock.Anything).Return(allContainersResponse, nil)
	api.EXPECT().ContainerInspect(mock.Anything, "foo").Return(DetailsResponse(AsMap()), errors.New("uh-oh"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	_, err := client.ListContainers(context.TODO(), mockAllContainers, ctr.ListOpts{All: true})

	assert.Error(t, err)
	assert.EqualError(t, err, "failed to inspect container: uh-oh")
	api.AssertExpectations(t)
}

func TestListContainers_InspectImageError(t *testing.T) {
	allContainersResponse := Containers(Response(AsMap(
		"ID", "foo",
		"Names", []string{"bar"})),
	)
	resp := DetailsResponse(AsMap("Image", "abc123"))
	imageDetailsResponse := ImageDetailsResponse(AsMap())
	api := NewMockEngine(t)
	api.EXPECT().ContainerList(mock.Anything, mock.Anything).Return(allContainersResponse, nil)
	api.EXPECT().ContainerInspect(mock.Anything, "foo").Return(resp, nil)
	api.EXPECT().ImageInspect(mock.Anything, "abc123").Return(imageDetailsResponse, errors.New("whoops"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	containers, err := client.ListContainers(context.TODO(), mockAllContainers, ctr.ListOpts{All: true})

	assert.NoError(t, err)
	assert.Len(t, containers, 1)
	api.AssertExpectations(t)
}

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
