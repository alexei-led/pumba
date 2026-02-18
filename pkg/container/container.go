package container

import (
	"fmt"
	"strings"
)

const (
	pumbaLabel     = "com.gaiaadm.pumba"
	pumbaSkipLabel = "com.gaiaadm.pumba.skip"
	signalLabel    = "com.gaiaadm.pumba.stop-signal"
	trueValue      = "true"

	// StateRunning represents a running container state.
	StateRunning = "running"
	// StateExited represents an exited container state.
	StateExited = "exited"
)

// NetworkLink represents a link from one container network endpoint.
type NetworkLink struct {
	Links []string
}

// Container represents a running container, decoupled from any specific runtime.
type Container struct {
	ContainerID   string
	ContainerName string
	Image         string
	ImageID       string
	State         string
	Labels        map[string]string
	Networks      map[string]NetworkLink
}

// ID returns the container ID.
func (c *Container) ID() string {
	return c.ContainerID
}

// Name returns the container name.
func (c *Container) Name() string {
	return c.ContainerName
}

// ImageName returns the name of the image that was used to start the
// container. If the original image was specified without a particular tag, the
// "latest" tag is assumed.
func (c *Container) ImageName() string {
	imageName := c.Image
	if !strings.Contains(imageName, ":") {
		imageName = fmt.Sprintf("%s:latest", imageName)
	}

	return imageName
}

// Links returns a list containing the names of all the containers to which
// this container is linked.
func (c *Container) Links() []string {
	var links []string

	for _, network := range c.Networks {
		for _, link := range network.Links {
			name := strings.Split(link, ":")[0]
			links = append(links, name)
		}
	}

	return links
}

// IsPumba returns a boolean flag indicating whether or not the current
// container is the Pumba container itself. The Pumba container is
// identified by the presence of the "com.gaiaadm.pumba" label in
// the container metadata.
func (c *Container) IsPumba() bool {
	val, ok := c.Labels[pumbaLabel]
	return ok && val == trueValue
}

// IsPumbaSkip returns a boolean flag indicating whether or not the current
// container should be ignored by Pumba. This container is
// identified by the presence of the "com.gaiaadm.pumba.skip" label in
// the container metadata. Use it to skip monitoring and helper containers.
func (c *Container) IsPumbaSkip() bool {
	val, ok := c.Labels[pumbaSkipLabel]
	return ok && val == trueValue
}

// StopSignal returns the custom stop signal (if any) that is encoded in the
// container's metadata. If the container has not specified a custom stop
// signal, the empty string "" is returned.
func (c *Container) StopSignal() string {
	if val, ok := c.Labels[signalLabel]; ok {
		return val
	}

	return ""
}
