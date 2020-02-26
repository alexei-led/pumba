package container

import (
	"fmt"
	"net"
	"strings"

	"github.com/docker/docker/api/types"
)

const (
	pumbaLabel     = "com.gaiaadm.pumba"
	pumbaSkipLabel = "com.gaiaadm.pumba.skip"
	signalLabel    = "com.gaiaadm.pumba.stop-signal"
)

type conn interface {
	net.Conn
}

// Container represents a running Docker container.
type Container struct {
	containerInfo types.ContainerJSON
	imageInfo     types.ImageInspect
}

// NewContainer returns a new Container instance instantiated with the
// specified ContainerJSON and ImageInpsect structs.
func NewContainer(containerInfo types.ContainerJSON, imageInfo types.ImageInspect) *Container {
	return &Container{
		containerInfo: containerInfo,
		imageInfo:     imageInfo,
	}
}

// ID returns the Docker container ID.
func (c Container) ID() string {
	return c.containerInfo.ID
}

// Name returns the Docker container name.
func (c Container) Name() string {
	return c.containerInfo.Name
}

// ImageID returns the ID of the Docker image that was used to start the container.
func (c Container) ImageID() string {
	return c.imageInfo.ID
}

// ImageName returns the name of the Docker image that was used to start the
// container. If the original image was specified without a particular tag, the
// "latest" tag is assumed.
func (c Container) ImageName() string {
	imageName := c.containerInfo.Image
	if !strings.Contains(imageName, ":") {
		imageName = fmt.Sprintf("%s:latest", imageName)
	}

	return imageName
}

// Links returns a list containing the names of all the containers to which
// this container is linked.
func (c Container) Links() []string {
	var links []string

	if c.containerInfo.NetworkSettings != nil {
		networkSettings := c.containerInfo.NetworkSettings
		for _, network := range networkSettings.Networks {
			for _, link := range network.Links {
				name := strings.Split(link, ":")[0]
				links = append(links, name)
			}
		}
	}

	return links
}

// IsPumba returns a boolean flag indicating whether or not the current
// container is the Pumba container itself. The Pumba container is
// identified by the presence of the "com.gaiaadm.pumba" label in
// the container metadata.
func (c Container) IsPumba() bool {
	val, ok := c.containerInfo.Config.Labels[pumbaLabel]
	return ok && val == "true"
}

// IsPumbaSkip returns a boolean flag indicating whether or not the current
// container should be ingored by Pumba. This container is
// identified by the presence of the "com.gaiaadm.pumba.skip" label in
// the container metadata. Use it to skip monitoring and helper containers.
func (c Container) IsPumbaSkip() bool {
	val, ok := c.containerInfo.Config.Labels[pumbaSkipLabel]
	return ok && val == "true"
}

// StopSignal returns the custom stop signal (if any) that is encoded in the
// container's metadata. If the container has not specified a custom stop
// signal, the empty string "" is returned.
func (c Container) StopSignal() string {
	if val, ok := c.containerInfo.Config.Labels[signalLabel]; ok {
		return val
	}

	return ""
}
