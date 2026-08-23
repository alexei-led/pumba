package netem

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

var (
	// DelayDistribution netem delay distributions
	delayDistribution = []string{"", "uniform", "normal", "pareto", "paretonormal"}
)

// `netem delay` command
type delayCommand struct {
	client       netemClient
	gp           *chaos.GlobalParams
	req          *container.NetemRequest
	limit        int
	time         int
	jitter       int
	correlation  float64
	distribution string
}

func validateDelay(delay, jitter int, correlation float64, distribution string) error {
	if delay <= 0 {
		return errors.New("non-positive delay time")
	}
	if jitter < 0 || jitter > delay {
		return errors.New("invalid delay jitter: must be non-negative and smaller than delay time")
	}
	if correlation < 0.0 || correlation > 100.0 {
		return errors.New("invalid delay correlation: must be between 0.0 and 100.0")
	}
	if !slices.Contains(delayDistribution, distribution) {
		return errors.New("invalid delay distribution: must be one of {uniform | normal | pareto |  paretonormal}")
	}
	return nil
}

func delayArgs(delay, jitter int, correlation float64, distribution string) []string {
	cmd := []string{"delay", strconv.Itoa(delay) + "ms"}
	if jitter > 0 {
		cmd = append(cmd, strconv.Itoa(jitter)+"ms")
	}
	if correlation > 0 {
		cmd = append(cmd, strconv.FormatFloat(correlation, 'f', 2, 64))
	}
	if distribution != "" {
		cmd = append(cmd, "distribution", distribution)
	}
	return cmd
}

// NewDelayCommand create new netem delay command
func NewDelayCommand(client netemClient,
	gp *chaos.GlobalParams,
	req *container.NetemRequest,
	limit int,
	delay, // delay time
	jitter int, // delay jitter
	correlation float64, // delay correlation
	distribution string, // delay distribution
) (chaos.Command, error) {
	if err := validateDelay(delay, jitter, correlation, distribution); err != nil {
		return nil, err
	}
	return &delayCommand{
		client:       client,
		gp:           gp,
		req:          req,
		limit:        limit,
		time:         delay,
		jitter:       jitter,
		correlation:  correlation,
		distribution: distribution,
	}, nil
}

// Run netem delay command
func (n *delayCommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network delay to all matching containers")
	log.WithFields(log.Fields{
		"names":   n.gp.Names,
		"pattern": n.gp.Pattern,
		"labels":  n.gp.Labels,
		"limit":   n.limit,
		"random":  random,
	}).Debug("listing matching containers")
	// Resolve --target container names/IDs once per command invocation,
	// before containers are enumerated, rather than once per matched
	// container inside the loop below.
	resolvedReq, err := resolveRequestTargets(ctx, n.client, n.req)
	if err != nil {
		return fmt.Errorf("failed to resolve --target: %w", err)
	}
	netemCmd := n.buildNetemCmd()
	return chaos.RunOnContainers(ctx, n.client, n.gp, n.limit, random, true,
		func(ctx context.Context, c *container.Container) error {
			log.WithFields(log.Fields{"container": c}).Debug("adding network delay for container")
			netemCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			req := *resolvedReq
			req.Container = c
			req.Command = netemCmd
			if err := runNetem(netemCtx, n.client, &req); err != nil {
				log.WithError(err).Warn("failed to delay network for container")
				return fmt.Errorf("failed to delay packets for one or more containers: %w", err)
			}
			return nil
		})
}

func (n *delayCommand) buildNetemCmd() []string {
	return delayArgs(n.time, n.jitter, n.correlation, n.distribution)
}
