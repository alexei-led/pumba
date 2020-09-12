package container

import (
	"fmt"
)

// CreateTestContainers create test container
func CreateTestContainers(count int) []*Container {
	containers := []*Container{}
	for i := 0; i < count; i++ {
		containers = append(containers, NewContainer(
			DetailsResponse(AsMap("Name", fmt.Sprintf("c%d", i))),
			ImageDetailsResponse(AsMap()),
		))
	}
	return containers
}

// CreateLabeledTestContainers generate test containers with labels
func CreateLabeledTestContainers(count int, labels map[string]string) []*Container {
	containers := []*Container{}
	for i := 0; i < count; i++ {
		containers = append(containers, NewContainer(
			DetailsResponse(AsMap("Name", fmt.Sprintf("c%d", i), "Labels", labels)),
			ImageDetailsResponse(AsMap()),
		))
	}
	return containers
}
