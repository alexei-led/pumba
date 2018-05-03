package container

import (
	"fmt"
)

func CreateTestContainers(count int) []Container {
	containers := []Container{}
	for i := 0; i < count; i++ {
		containers = append(containers, *NewContainer(
			ContainerDetailsResponse(AsMap("Name", fmt.Sprintf("c%d", i))),
			ImageDetailsResponse(AsMap()),
		))
	}
	return containers
}
