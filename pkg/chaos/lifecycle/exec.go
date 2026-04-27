package lifecycle

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
	gp := &chaos.GlobalParams{Names: k.names, Pattern: k.pattern, Labels: k.labels}
	return chaos.RunOnContainers(ctx, k.client, gp, k.limit, random, false,
		func(ctx context.Context, c *container.Container) error {
			log.WithFields(log.Fields{"c": *c, "command": k.command, "args": k.args}).Debug("execing c")
			if err := k.client.ExecContainer(ctx, c, k.command, k.args, k.dryRun); err != nil {
				return fmt.Errorf("failed to run exec command: %w", err)
			}
			return nil
		})
}
