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

// `netem loss state` command
type lossStateCommand struct {
	client netemClient
	gp     *chaos.GlobalParams
	req    *container.NetemRequest
	limit  int
	p13    float64
	p31    float64
	p32    float64
	p23    float64
	p14    float64
}

// NewLossStateCommand create new netem loss state command
func NewLossStateCommand(client netemClient,
	gp *chaos.GlobalParams,
	req *container.NetemRequest,
	limit int,
	p13, // probability to go from state (1) to state (3)
	p31, // probability to go from state (3) to state (1)
	p32, // probability to go from state (3) to state (2)
	p23, // probability to go from state (2) to state (3)
	p14 float64, // probability to go from state (1) to state (4)
) (chaos.Command, error) {
	// validate p13
	if p13 < 0.0 || p13 > 100.0 {
		return nil, errors.New("invalid p13 percentage: : must be between 0.0 and 100.0")
	}
	// validate p31
	if p31 < 0.0 || p31 > 100.0 {
		return nil, errors.New("invalid p31 percentage: : must be between 0.0 and 100.0")
	}
	// validate p32
	if p32 < 0.0 || p32 > 100.0 {
		return nil, errors.New("invalid p32 percentage: : must be between 0.0 and 100.0")
	}
	// vaidate p23
	if p23 < 0.0 || p23 > 100.0 {
		return nil, errors.New("invalid p23 percentage: : must be between 0.0 and 100.0")
	}
	// validate p14
	if p14 < 0.0 || p14 > 100.0 {
		return nil, errors.New("invalid p14 percentage: : must be between 0.0 and 100.0")
	}

	return &lossStateCommand{
		client: client,
		gp:     gp,
		req:    req,
		limit:  limit,
		p13:    p13,
		p31:    p31,
		p32:    p32,
		p23:    p23,
		p14:    p14,
	}, nil
}

// Run netem loss state command
//
//nolint:dupl
func (n *lossStateCommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network packet loss according 4-state Markov model to all matching containers")
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
			log.WithFields(log.Fields{"container": c}).Debug("adding network 4-state packet loss for container")
			netemCtx, cancel := context.WithTimeout(ctx, n.req.Duration)
			defer cancel()
			req := *n.req
			req.Container = c
			req.Command = netemCmd
			if err := runNetem(netemCtx, n.client, &req); err != nil {
				log.WithError(err).Warn("failed to set packet loss for container")
				return fmt.Errorf("failed to add packet loss (4-state Markov model) for one or more containers: %w", err)
			}
			return nil
		})
}

func (n *lossStateCommand) buildNetemCmd() []string {
	cmd := []string{"loss", "state", strconv.FormatFloat(n.p13, 'f', 2, 64)}
	cmd = append(cmd,
		strconv.FormatFloat(n.p31, 'f', 2, 64),
		strconv.FormatFloat(n.p32, 'f', 2, 64),
		strconv.FormatFloat(n.p23, 'f', 2, 64),
		strconv.FormatFloat(n.p14, 'f', 2, 64))
	return cmd
}
