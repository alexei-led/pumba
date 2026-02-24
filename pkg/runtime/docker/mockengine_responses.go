package docker

import (
	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	networktypes "github.com/docker/docker/api/types/network"
)

// Containers list of containers
func Containers(containers ...ctypes.Summary) []ctypes.Summary {
	return containers
}

// Response mock single container
func Response(params map[string]any) ctypes.Summary {
	ID := lookupWithDefault(params, "ID", "defaultID").(string)
	Names := lookupWithDefault(params, "Names", []string{"foo", "bar"}).([]string)

	return ctypes.Summary{
		ID:    ID,
		Names: Names,
	}
}

// DetailsResponse mock container details response
func DetailsResponse(params map[string]any) ctypes.InspectResponse {
	ID := lookupWithDefault(params, "ID", "defaultID").(string)
	Name := lookupWithDefault(params, "Name", "defaultName").(string)
	Created := lookupWithDefault(params, "Created", "2015-07-01T12:00:01.000000000Z").(string)
	Image := lookupWithDefault(params, "Image", "defaultImage").(string)
	Running := lookupWithDefault(params, "Running", false).(bool)
	Labels := lookupWithDefault(params, "Labels", map[string]string{}).(map[string]string)
	Links := lookupWithDefault(params, "Links", []string{}).([]string)
	CgroupParent := lookupWithDefault(params, "CgroupParent", "").(string)

	resp := ctypes.InspectResponse{
		ContainerJSONBase: &ctypes.ContainerJSONBase{
			ID:      ID,
			Name:    Name,
			Created: Created,
			Image:   Image,
			State:   &ctypes.State{Running: Running},
			HostConfig: &ctypes.HostConfig{
				Resources: ctypes.Resources{
					CgroupParent: CgroupParent,
				},
			},
		},
		Config: &ctypes.Config{
			Labels: Labels,
		},
		NetworkSettings: &ctypes.NetworkSettings{
			Networks: map[string]*networktypes.EndpointSettings{
				"default": {Links: Links},
			},
		},
	}
	return resp
}

// ImageDetailsResponse mock image response
func ImageDetailsResponse(params map[string]any) imagetypes.InspectResponse {
	ID := lookupWithDefault(params, "ID", "defaultID").(string)

	return imagetypes.InspectResponse{
		ID: ID,
	}
}

// NewTestContainer creates a Container directly from params for testing
func NewTestContainer(params map[string]any) *ctr.Container {
	id := lookupWithDefault(params, "ID", "defaultID").(string)
	name := lookupWithDefault(params, "Name", "defaultName").(string)
	image := lookupWithDefault(params, "Image", "defaultImage").(string)
	labels := lookupWithDefault(params, "Labels", map[string]string{}).(map[string]string)
	links := lookupWithDefault(params, "Links", []string{}).([]string)
	state := ctr.StateRunning
	if running, ok := params["Running"]; ok && !running.(bool) {
		state = ctr.StateExited
	}
	networks := map[string]ctr.NetworkLink{}
	if len(links) > 0 {
		networks["default"] = ctr.NetworkLink{Links: links}
	}
	return &ctr.Container{
		ContainerID:   id,
		ContainerName: name,
		Image:         image,
		ImageID:       lookupWithDefault(params, "ImageID", "defaultID").(string),
		State:         state,
		Labels:        labels,
		Networks:      networks,
	}
}

func lookupWithDefault(aMap map[string]any, key string, defaultValue any) any {
	if value, present := aMap[key]; present {
		return value
	}
	return defaultValue
}

// AsMap convert multiple arguments into map[string]any
func AsMap(args ...any) map[string]any {
	paramMap := make(map[string]any)
	for i := 0; i+1 < len(args); i += 2 {
		paramMap[args[i].(string)] = args[i+1]
	}
	return paramMap
}
