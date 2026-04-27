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
	gp := &chaos.GlobalParams{Names: r.names, Pattern: r.pattern, Labels: r.labels}
	return chaos.RunOnContainersAll(ctx, r.client, gp, r.limit, random, false,
		func(ctx context.Context, c *container.Container) error {
			log.WithFields(log.Fields{
				"container": c,
				"force":     r.opts.Force,
				"links":     r.opts.Links,
				"volumes":   r.opts.Volumes,
			}).Debug("removing container")
			if err := r.client.RemoveContainer(ctx, c, r.opts); err != nil {
				return fmt.Errorf("failed to remove container: %w", err)
			}
			return nil
		})
}
