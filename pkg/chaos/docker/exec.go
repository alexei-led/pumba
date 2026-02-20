package docker

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// execClient is the narrow interface needed by the exec command.
type execClient interface {
	container.Lister
	container.Executor
}

// `docker exec` command
type execCommand struct {
	client  execClient
	names   []string
	pattern string
	labels  []string
	command string
	args    []string
	limit   int
	dryRun  bool
}

// NewExecCommand create new Exec Command instance
func NewExecCommand(client execClient, params *chaos.GlobalParams, command string, args []string, limit int) chaos.Command {
	exec := &execCommand{
		client:  client,
		names:   params.Names,
		pattern: params.Pattern,
		labels:  params.Labels,
		command: command,
		args:    args,
		limit:   limit,
		dryRun:  params.DryRun,
	}
	if exec.command == "" {
		exec.command = "kill 1"
	}
	return exec
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
		return fmt.Errorf("error listing containers: %w", err)
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
			"args":    k.args,
		}).Debug("execing c")
		cc := c
		err = k.client.ExecContainer(ctx, cc, k.command, k.args, k.dryRun)
		if err != nil {
			return fmt.Errorf("failed to run exec command: %w", err)
		}
	}
	return nil
}
