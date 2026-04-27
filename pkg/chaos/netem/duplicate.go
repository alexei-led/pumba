package netem

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

const (
	duplicateCmd = "duplicate"
)

// `netem duplicate` command
type duplicateCommand struct {
	client      netemClient
	gp          *chaos.GlobalParams
	req         *container.NetemRequest
	limit       int
	percent     float64
	correlation float64
}

// NewDuplicateCommand create new netem duplicate command
func NewDuplicateCommand(client netemClient,
	gp *chaos.GlobalParams,
	req *container.NetemRequest,
	limit int,
	percent, // duplicate percent
	correlation float64, // duplicate correlation
) (chaos.Command, error) {
	// get netem duplicate percent
	if percent < 0.0 || percent > 100.0 {
		return nil, errors.New("invalid duplicate percent: must be between 0.0 and 100.0")
	}
	// get netem duplicate variation
	if correlation < 0.0 || correlation > 100.0 {
		return nil, errors.New("invalid duplicate correlation: must be between 0.0 and 100.0")
	}
	return &duplicateCommand{
		client:      client,
		gp:          gp,
		req:         req,
		limit:       limit,
		percent:     percent,
		correlation: correlation,
	}, nil
}

// Run netem duplicate command
//
//nolint:dupl
func (n *duplicateCommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network random packet duplicates to all matching containers")
	log.WithFields(log.Fields{
		"names":   n.gp.Names,
		"pattern": n.gp.Pattern,
		"labels":  n.gp.Labels,
		"limit":   n.limit,
		"random":  random,
	}).Debug("listing matching containers")
	netemCmd := n.buildNetemCmd()
	return chaos.RunOnContainers(ctx, n.client, n.gp, n.limit, random, true,
		func(ctx context.Context, c *container.Container) error {
			log.WithFields(log.Fields{"container": c}).Debug("adding network random packet duplicates for container")
			netemCtx, cancel := context.WithTimeout(ctx, n.req.Duration)
			defer cancel()
			req := *n.req
			req.Container = c
			req.Command = netemCmd
			if err := runNetem(netemCtx, n.client, &req); err != nil {
				log.WithError(err).Warn("failed to set packet duplicates for container")
				return fmt.Errorf("failed to set packet duplicates for one or more containers: %w", err)
			}
			return nil
		})
}

func (n *duplicateCommand) buildNetemCmd() []string {
	cmd := []string{duplicateCmd, strconv.FormatFloat(n.percent, 'f', 2, 64)}
	if n.correlation > 0 {
		cmd = append(cmd, strconv.FormatFloat(n.correlation, 'f', 2, 64))
	}
	return cmd
}
