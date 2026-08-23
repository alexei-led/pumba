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

// `netem corrupt` command
type corruptCommand struct {
	client      netemClient
	gp          *chaos.GlobalParams
	req         *container.NetemRequest
	limit       int
	percent     float64
	correlation float64
}

func validateCorrupt(percent, correlation float64) error {
	if percent < 0.0 || percent > 100.0 {
		return errors.New("invalid corrupt percent: must be between 0.0 and 100.0")
	}
	if correlation < 0.0 || correlation > 100.0 {
		return errors.New("invalid corrupt correlation: must be between 0.0 and 100.0")
	}
	return nil
}

func corruptArgs(percent, correlation float64) []string {
	cmd := []string{"corrupt", strconv.FormatFloat(percent, 'f', 2, 64)}
	if correlation > 0 {
		cmd = append(cmd, strconv.FormatFloat(correlation, 'f', 2, 64))
	}
	return cmd
}

// NewCorruptCommand create new netem corrupt command
func NewCorruptCommand(client netemClient,
	gp *chaos.GlobalParams,
	req *container.NetemRequest,
	limit int,
	percent, // corrupt percent
	correlation float64, // corrupt correlation
) (chaos.Command, error) {
	if err := validateCorrupt(percent, correlation); err != nil {
		return nil, err
	}
	return &corruptCommand{
		client:      client,
		gp:          gp,
		req:         req,
		limit:       limit,
		percent:     percent,
		correlation: correlation,
	}, nil
}

// Run netem corrupt command
//
//nolint:dupl
func (n *corruptCommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network random packet corrupt to all matching containers")
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
			log.WithFields(log.Fields{"container": c}).Debug("adding network random packet corrupt for container")
			netemCtx, cancel := context.WithTimeout(ctx, n.req.Duration)
			defer cancel()
			req := *resolvedReq
			req.Container = c
			req.Command = netemCmd
			if err := runNetem(netemCtx, n.client, &req); err != nil {
				log.WithError(err).Warn("failed to set packet corrupt for container")
				return fmt.Errorf("failed to corrupt packets for one or more containers: %w", err)
			}
			return nil
		})
}

func (n *corruptCommand) buildNetemCmd() []string {
	return corruptArgs(n.percent, n.correlation)
}
