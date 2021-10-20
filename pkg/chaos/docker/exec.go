package docker

import (
	"context"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// ExecCommand `docker exec` command
type ExecCommand struct {
	client  container.Client
	names   []string
	pattern string
	labels  []string
	command string
	limit   int
	dryRun  bool
}

// NewExecCommand create new Exec Command instance
func NewExecCommand(client container.Client, names []string, pattern string, labels []string, command string, limit int, dryRun bool) (chaos.Command, error) {
	exec := &ExecCommand{client, names, pattern, labels, command, limit, dryRun}
	if exec.command == "" {
		exec.command = "kill 1"
	}
	return exec, nil
}

// Run exec command
func (k *ExecCommand) Run(ctx context.Context, random bool) error {
	log.Debug("execing all matching containers")
	log.WithFields(log.Fields{
		"names":   k.names,
		"pattern": k.pattern,
		"labels":  k.labels,
		"limit":   k.limit,
		"random":  random,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, k.client, k.names, k.pattern, k.labels, k.limit)
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		log.Warning("no containers to exec")
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
			"command":   k.command,
		}).Debug("execing container")
		c := container
		err = k.client.ExecContainer(ctx, c, k.command, k.dryRun)
		if err != nil {
			return errors.Wrap(err, "failed to exec container")
		}
	}
	return nil
}
