package container

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
)

const (
	pumbaLabel     = "com.gaiaadm.pumba"
	pumbaSkipLabel = "com.gaiaadm.pumba.skip"
	signalLabel    = "com.gaiaadm.pumba.stop-signal"
	trueValue      = "true"
)

// Container represents a running container (Docker or containerd).
type Container struct {
	// Docker-specific info (remains for compatibility with dockerClient)
	ContainerInfo types.ContainerJSON // Might be nil if using containerd
	ImageInfo     types.ImageInspect  // Might be nil if using containerd

	// Common fields populated by both Docker and containerd clients
	Cid         string
	Cname       string
	Clabels     map[string]string
	CimageName  string
	Cstatus     string
	CstopSignal string // Custom stop signal from labels
}

// ID returns the container ID.
func (c *Container) ID() string {
	if c.Cid != "" {
		return c.Cid
	}
	if c.ContainerInfo.ContainerJSONBase != nil {
		return c.ContainerInfo.ID
	}
	return ""
}

// Name returns the container name.
// For Docker, it's the explicit name. For containerd, it might be the ID or a specific label.
func (c *Container) Name() string {
	if c.Cname != "" {
		return c.Cname
	}
	if c.ContainerInfo.ContainerJSONBase != nil {
		return c.ContainerInfo.Name
	}
	// Fallback for containerd if Cname is not explicitly set to ID or a friendly name
	return c.ID()
}

// ImageID returns the ID of the Docker image that was used to start the container.
// This is Docker-specific. containerd might not always have a direct equivalent easily accessible or used by Pumba.
func (c *Container) ImageID() string {
	if c.ImageInfo.ID != "" { // Check if ImageInfo is not the zero struct
		return c.ImageInfo.ID
	}
	return ""
}

// ImageName returns the name of the image used to start the container.
func (c *Container) ImageName() string {
	if c.CimageName != "" {
		return c.CimageName
	}
	if c.ContainerInfo.ContainerJSONBase != nil {
		imageName := c.ContainerInfo.Image
		if !strings.Contains(imageName, ":") && imageName != "" { // imageName can be a sha256 hash for docker
			// Only append :latest if it's not an image ID/hash
			if !strings.HasPrefix(imageName, "sha256:") && !isHex(imageName) { // Basic check if it's an ID
				imageName = fmt.Sprintf("%s:latest", imageName)
			}
		}
		return imageName
	}
	return ""
}

// isHex checks if a string is likely a hex identifier (like an image ID).
func isHex(s string) bool {
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return len(s) > 0 // e.g. Docker image IDs are 64 hex chars
}


// Links returns a list containing the names of all the containers to which
// this container is linked. This is Docker-specific.
func (c *Container) Links() []string {
	var links []string
	if c.ContainerInfo.NetworkSettings != nil {
		networkSettings := c.ContainerInfo.NetworkSettings
		for _, network := range networkSettings.Networks {
			for _, link := range network.Links {
				name := strings.Split(link, ":")[0]
				links = append(links, name)
			}
		}
	}
	return links
}

// Labels returns the container's labels.
func (c *Container) Labels() map[string]string {
	if len(c.Clabels) > 0 {
		return c.Clabels
	}
	if c.ContainerInfo.Config != nil {
		return c.ContainerInfo.Config.Labels
	}
	return make(map[string]string)
}

// IsPumba returns a boolean flag indicating whether or not the current
// container is the Pumba container itself.
func (c *Container) IsPumba() bool {
	labels := c.Labels()
	val, ok := labels[pumbaLabel]
	return ok && val == trueValue
}

// IsPumbaSkip returns a boolean flag indicating whether or not the current
// container should be ignored by Pumba.
func (c *Container) IsPumbaSkip() bool {
	labels := c.Labels()
	val, ok := labels[pumbaSkipLabel]
	return ok && val == trueValue
}

// Status returns the container's status string.
func (c *Container) Status() string {
	if c.Cstatus != "" {
		return c.Cstatus
	}
	if c.ContainerInfo.State != nil {
		return c.ContainerInfo.State.Status
	}
	return ""
}

// StopSignal returns the custom stop signal (if any) that is encoded in the
// container's metadata. If the container has not specified a custom stop
// signal, the empty string "" is returned.
func (c *Container) StopSignal() string {
	if c.CstopSignal != "" {
		return c.CstopSignal
	}
	labels := c.Labels()
	if val, ok := labels[signalLabel]; ok {
		return val
	}
	return ""
}
