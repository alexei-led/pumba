package docker

import (
	"github.com/alexei-led/pumba/pkg/container"
)

var testContainer3 = []container.Container{
	*container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "c1")),
		container.ImageDetailsResponse(container.AsMap()),
	),
	*container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "c2")),
		container.ImageDetailsResponse(container.AsMap()),
	),
	*container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "c3")),
		container.ImageDetailsResponse(container.AsMap()),
	),
}
