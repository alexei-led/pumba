package container

import (
	"bytes"
	"fmt"
	"io"
)

// CreateTestContainers create test container
func CreateTestContainers(count int) []*Container {
	var containers []*Container
	for i := 0; i < count; i++ {
		containers = append(containers, &Container{
			DetailsResponse(AsMap("Name", fmt.Sprintf("c%d", i))),
			ImageDetailsResponse(AsMap()),
		})
	}
	return containers
}

// CreateLabeledTestContainers generate test containers with labels
func CreateLabeledTestContainers(count int, labels map[string]string) []*Container {
	var containers []*Container
	for i := 0; i < count; i++ {
		containers = append(containers, &Container{
			DetailsResponse(AsMap("Name", fmt.Sprintf("c%d", i), "Labels", labels)),
			ImageDetailsResponse(AsMap()),
		})
	}
	return containers
}

// Wrap wraps a given text reader with a ReadCloser
func Wrap(text string) io.ReadCloser {
	return io.NopCloser(bytes.NewReader([]byte(text)))
}

// DockerAPIResponse docker container api response body
type DockerAPIResponse struct {
	Container string
	File      string
	Status    string
}
