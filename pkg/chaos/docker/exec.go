package docker

import (
	"context"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// `docker exec` command
type execCommand struct {
	client  container.Client
	names   []string
	pattern string
	labels  []string
	command string
	limit   int
	dryRun  bool
}

// NewExecCommand create new Exec Command instance
func NewExecCommand(client container.Client, params *chaos.GlobalParams, command string, limit int) (chaos.Command, error) {
	exec := &execCommand{
		client:  client,
		names:   params.Names,
		pattern: params.Pattern,
		labels:  params.Labels,
		command: command,
		limit:   limit,
		dryRun:  params.DryRun,
	}
	if exec.command == "" {
		exec.command = "kill 1"
	}
	return exec, nil
}

// Run exec command
func (k *execCommand) Run(ctx context.Context, random bool) error {
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

	// select single random c from matching c and replace list with selected item
	if random {
		if c := container.RandomContainer(containers); c != nil {
			containers = []*container.Container{c}
		}
	}
	for _, c := range containers {
		log.WithFields(log.Fields{
			"c":       *c,
			"command": k.command,
		}).Debug("execing c")
		cc := c
		err = k.client.ExecContainer(ctx, cc, k.command, k.dryRun)
		if err != nil {
			return errors.Wrap(err, "failed to run exec command")
		}
	}
	return nil
}
