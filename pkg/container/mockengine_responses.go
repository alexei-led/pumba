package container

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

func Containers(containers... types.Container) []types.Container {
	return containers;
}

func ContainerResponse(params map[string]interface{}) types.Container {
	ID := lookupWithDefault(params, "ID", "defaultID").(string)
	Names := lookupWithDefault(params, "Names", []string{"foo", "bar"}).([]string)

	return types.Container{
		ID: ID,
		Names: Names,
	}
}

func ContainerDetailsResponse(params map[string]interface{}) types.ContainerJSON {
	ID := lookupWithDefault(params, "ID", "defaultID").(string)
	Name := lookupWithDefault(params, "Name", "defaultName").(string)
	Created := lookupWithDefault(params, "Created", "2015-07-01T12:00:01.000000000Z").(string)
	Image := lookupWithDefault(params, "Image", "defaultImage").(string)
	Running := lookupWithDefault(params, "Running", false).(bool)
	Labels := lookupWithDefault(params, "Labels", map[string]string{}).(map[string]string)
	Links := lookupWithDefault(params, "Links", []string{}).([]string)

	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: ID,
			Name: Name,
			Created: Created,
			Image: Image,
			State: &types.ContainerState{Running: Running},
		},
		Config: &container.Config{
			Labels: Labels,
		},
		NetworkSettings: &types.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"default": {Links: Links},
			},
		},
	}
}

func ImageDetailsResponse(params map[string]interface{}) types.ImageInspect {
	ID := lookupWithDefault(params, "ID", "defaultID").(string)

	return types.ImageInspect{
		ID: ID,
	}
}

func lookupWithDefault(aMap map[string]interface{}, key string, defaultValue interface{}) interface{} {
	if value, present := aMap[key]; present {
		return value
	}
	return defaultValue
}

func AsMap(args ... interface{}) map[string]interface{} {
	paramMap := make(map[string]interface{})
	for i := 0; i < len(args); i += 2 {
		paramMap[args[i].(string)] = args[i + 1]
	}
	return paramMap
}

