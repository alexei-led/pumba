package docker

import (
	"context"
	"fmt"

	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	imagetypes "github.com/docker/docker/api/types/image"
	log "github.com/sirupsen/logrus"
)

// dockerInspectToContainer converts Docker inspect responses into a runtime-agnostic Container.
func dockerInspectToContainer(info ctypes.InspectResponse, img *imagetypes.InspectResponse) *ctr.Container {
	c := &ctr.Container{
		ImageID:  img.ID,
		Labels:   make(map[string]string),
		Networks: make(map[string]ctr.NetworkLink),
	}
	if info.ContainerJSONBase != nil {
		c.ContainerID = info.ID
		c.ContainerName = info.Name
		c.Image = info.Image
		if info.State != nil {
			if info.State.Running {
				c.State = ctr.StateRunning
			} else {
				c.State = ctr.StateExited
			}
		}
	}
	if info.Config != nil && info.Config.Labels != nil {
		c.Labels = info.Config.Labels
	}
	if info.NetworkSettings != nil {
		for name, ep := range info.NetworkSettings.Networks {
			c.Networks[name] = ctr.NetworkLink{Links: ep.Links}
		}
	}
	return c
}

// ListContainers returns a list of containers that match the given filter
func (client dockerClient) ListContainers(ctx context.Context, fn ctr.FilterFunc, opts ctr.ListOpts) ([]*ctr.Container, error) {
	filterArgs := filters.NewArgs()
	for _, label := range opts.Labels {
		filterArgs.Add("label", label)
	}
	return client.listContainers(ctx, fn, ctypes.ListOptions{All: opts.All, Filters: filterArgs})
}

func (client dockerClient) listContainers(ctx context.Context, fn ctr.FilterFunc, opts ctypes.ListOptions) ([]*ctr.Container, error) {
	log.Debug("listing containers")
	containers, err := client.containerAPI.ContainerList(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	var cs []*ctr.Container
	for _, container := range containers {
		containerInfo, err := client.containerAPI.ContainerInspect(ctx, container.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect container: %w", err)
		}
		log.WithFields(log.Fields{
			"name": containerInfo.Name,
			"id":   containerInfo.ID,
		}).Debug("found container")

		imgInfo, err := client.imageAPI.ImageInspect(ctx, containerInfo.Image)
		if err != nil {
			log.WithError(err).WithField("image", containerInfo.Image).Warn("failed to inspect container image, skipping image metadata")
		}
		c := dockerInspectToContainer(containerInfo, &imgInfo)
		if fn(c) {
			cs = append(cs, c)
		}
	}
	return cs, nil
}
