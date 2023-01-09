package container

import (
	"fmt"
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
