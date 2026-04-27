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

// netem loss gemodel` (Gilbert-Elliot model) command
type lossGECommand struct {
	client netemClient
	gp     *chaos.GlobalParams
	req    *container.NetemRequest
	limit  int
	pg     float64
	pb     float64
	oneH   float64
	oneK   float64
}

// NewLossGECommand create new netem loss gemodel (Gilbert-Elliot) command
func NewLossGECommand(client netemClient,
	gp *chaos.GlobalParams,
	req *container.NetemRequest,
	limit int,
	pg, // Good State transition probability
	pb, // Bad State transition probability
	oneH, // loss probability in Bad state
	oneK float64, // loss probability in Good state
) (chaos.Command, error) {
	// get pg - Good State transition probability
	if pg < 0.0 || pg > 100.0 {
		return nil, errors.New("invalid pg (Good State) transition probability: must be between 0.0 and 100.0")
	}
	// get pb - Bad State transition probability
	if pb < 0.0 || pb > 100.0 {
		return nil, errors.New("invalid pb (Bad State) transition probability: must be between 0.0 and 100.0")
	}
	// get (1-h) - loss probability in Bad state
	if oneH < 0.0 || oneH > 100.0 {
		return nil, errors.New("invalid loss probability: must be between 0.0 and 100.0")
	}
	// get (1-k) - loss probability in Good state
	if oneK < 0.0 || oneK > 100.0 {
		return nil, errors.New("invalid loss probability: must be between 0.0 and 100.0")
	}

	return &lossGECommand{
		client: client,
		gp:     gp,
		req:    req,
		limit:  limit,
		pg:     pg,
		pb:     pb,
		oneH:   oneH,
		oneK:   oneK,
	}, nil
}

// Run netem loss state command
//
//nolint:dupl
func (n *lossGECommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network packet loss according Gilbert-Elliot model to all matching containers")
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
			log.WithFields(log.Fields{"container": c}).Debug("adding network random packet loss for container")
			netemCtx, cancel := context.WithTimeout(ctx, n.req.Duration)
			defer cancel()
			req := *n.req
			req.Container = c
			req.Command = netemCmd
			if err := runNetem(netemCtx, n.client, &req); err != nil {
				log.WithError(err).Warn("failed to set packet loss for container")
				return fmt.Errorf("failed to add packet loss (Gilbert-Elliot model) for one or more containers: %w", err)
			}
			return nil
		})
}

func (n *lossGECommand) buildNetemCmd() []string {
	cmd := []string{"loss", "gemodel", strconv.FormatFloat(n.pg, 'f', 2, 64)}
	cmd = append(cmd,
		strconv.FormatFloat(n.pb, 'f', 2, 64),
		strconv.FormatFloat(n.oneH, 'f', 2, 64),
		strconv.FormatFloat(n.oneK, 'f', 2, 64))
	return cmd
}
