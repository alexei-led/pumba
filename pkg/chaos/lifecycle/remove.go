package lifecycle

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// removeClient is the narrow interface needed by the remove command.
type removeClient interface {
	container.Lister
	RemoveContainer(context.Context, *container.Container, container.RemoveOpts) error
}

// `docker rm` command
type removeCommand struct {
	client  removeClient
	names   []string
	pattern string
	labels  []string
	opts    container.RemoveOpts
	limit   int
}

// NewRemoveCommand create new Kill Command instance
func NewRemoveCommand(client removeClient, params *chaos.GlobalParams, force, links, volumes bool, limit int) chaos.Command {
	remove := &removeCommand{
		client:  client,
		names:   params.Names,
		pattern: params.Pattern,
		labels:  params.Labels,
		opts: container.RemoveOpts{
			Force:   force,
			Links:   links,
			Volumes: volumes,
			DryRun:  params.DryRun,
		},
		limit: limit,
	}
	return remove
}

// Run remove command
func (r *removeCommand) Run(ctx context.Context, random bool) error {
	log.Debug("removing all matching containers")
	log.WithFields(log.Fields{
		"names":   r.names,
		"pattern": r.pattern,
		"labels":  r.labels,
		"limit":   r.limit,
		"random":  random,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainersAll(ctx, r.client, r.names, r.pattern, r.labels, r.limit, true)
	if err != nil {
		return fmt.Errorf("error listing containers: %w", err)
	}
	if len(containers) == 0 {
		log.Warning("no containers to remove")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		if c := container.RandomContainer(containers); c != nil {
			containers = []*container.Container{c}
		}
	}

	for _, container := range containers {
		log.WithFields(log.Fields{
			"container": container,
			"force":     r.opts.Force,
			"links":     r.opts.Links,
			"volumes":   r.opts.Volumes,
		}).Debug("removing container")
		c := container
		err = r.client.RemoveContainer(ctx, c, r.opts)
		if err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}
	}
	return nil
}
