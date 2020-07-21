package docker

import (
	"context"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// RemoveCommand `docker kill` command
type RemoveCommand struct {
	client  container.Client
	names   []string
	pattern string
	labels  []string
	force   bool
	links   bool
	volumes bool
	limit   int
	dryRun  bool
}

// NewRemoveCommand create new Kill Command instance
func NewRemoveCommand(client container.Client, names []string, pattern string, labels []string, force bool, links bool, volumes bool, limit int, dryRun bool) (chaos.Command, error) {
	remove := &RemoveCommand{client, names, pattern, labels, force, links, volumes, limit, dryRun}
	return remove, nil
}

// Run remove command
func (r *RemoveCommand) Run(ctx context.Context, random bool) error {
	log.Debug("removing all matching containers")
	log.WithFields(log.Fields{
		"names":   r.names,
		"pattern": r.pattern,
		"labels":  r.labels,
		"limit":   r.limit,
		"random":  random,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, r.client, r.names, r.pattern, r.labels, r.limit)
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		log.Warning("no containers to remove")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		if c := container.RandomContainer(containers); c != nil {
			containers = []container.Container{*c}
		}
	}

	for _, container := range containers {
		log.WithFields(log.Fields{
			"container": container,
			"force":     r.force,
			"links":     r.links,
			"volumes":   r.volumes,
		}).Debug("removing container")
		c := container
		err = r.client.RemoveContainer(ctx, &c, r.force, r.links, r.volumes, r.dryRun)
		if err != nil {
			return err
		}
	}
	return nil
}
