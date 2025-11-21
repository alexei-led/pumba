package container

import (
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	networktypes "github.com/docker/docker/api/types/network"
)

// Containers list of containers
func Containers(containers ...ctypes.Summary) []ctypes.Summary {
	return containers
}

// Response mock single container
func Response(params map[string]interface{}) ctypes.Summary {
	ID := lookupWithDefault(params, "ID", "defaultID").(string)
	Names := lookupWithDefault(params, "Names", []string{"foo", "bar"}).([]string)

	return ctypes.Summary{
		ID:    ID,
		Names: Names,
	}
}

// DetailsResponse mock container details response
func DetailsResponse(params map[string]interface{}) ctypes.InspectResponse {
	ID := lookupWithDefault(params, "ID", "defaultID").(string)
	Name := lookupWithDefault(params, "Name", "defaultName").(string)
	Created := lookupWithDefault(params, "Created", "2015-07-01T12:00:01.000000000Z").(string)
	Image := lookupWithDefault(params, "Image", "defaultImage").(string)
	Running := lookupWithDefault(params, "Running", false).(bool)
	Labels := lookupWithDefault(params, "Labels", map[string]string{}).(map[string]string)
	Links := lookupWithDefault(params, "Links", []string{}).([]string)

	return ctypes.InspectResponse{
		ContainerJSONBase: &ctypes.ContainerJSONBase{
			ID:      ID,
			Name:    Name,
			Created: Created,
			Image:   Image,
			State:   &ctypes.State{Running: Running},
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
}

// ImageDetailsResponse mock image response
func ImageDetailsResponse(params map[string]interface{}) imagetypes.InspectResponse {
	ID := lookupWithDefault(params, "ID", "defaultID").(string)

	return imagetypes.InspectResponse{
		ID: ID,
	}
}

func lookupWithDefault(aMap map[string]interface{}, key string, defaultValue interface{}) interface{} {
	if value, present := aMap[key]; present {
		return value
	}
	return defaultValue
}

// AsMap convert multiple arguments into map[string]interface{}
func AsMap(args ...interface{}) map[string]interface{} {
	paramMap := make(map[string]interface{})
	for i := 0; i < len(args); i += 2 {
		paramMap[args[i].(string)] = args[i+1]
	}
	return paramMap
}
