package iptables

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// `iptables loss` command
type lossCommand struct {
	client      iptablesClient
	gp          *chaos.GlobalParams
	req         *container.IPTablesRequest
	iface       string
	protocol    string
	limit       int
	mode        string
	probability float64
	every       int
	packet      int
}

const (
	ModeRandom = "random"
	ModeNTH    = "nth"
)

// NewLossCommand create new iptables loss command
func NewLossCommand(client iptablesClient,
	gp *chaos.GlobalParams,
	base *RequestBase,
	mode string, // loss mode
	probability float64, // loss probability
	every int, // drop every nth
	packet int, // start budget for every nth
) (chaos.Command, error) {
	// get mode
	switch mode {
	case ModeRandom:
		// get loss probability
		if probability < 0.0 || probability > 1.0 {
			return nil, errors.New("invalid loss probability: must be between 0.0 and 1.0")
		}
	case ModeNTH:
		// get every
		if every <= 0 {
			return nil, errors.New("invalid loss every: must be > 0")
		}
		// get packet
		if packet < 0 || (packet > every-1) {
			return nil, errors.New("invalid loss packet: must be 0 <= packet <= every-1")
		}
	default:
		return nil, errors.New("invalid loss mode: must be either random or nth")
	}

	return &lossCommand{
		client:      client,
		gp:          gp,
		req:         base.Request,
		iface:       base.Iface,
		protocol:    base.Protocol,
		limit:       base.Limit,
		mode:        mode,
		probability: probability,
		every:       every,
		packet:      packet,
	}, nil
}

// Run iptables loss command
func (n *lossCommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network random packet loss to all matching containers")
	log.WithFields(log.Fields{
		"names":   n.gp.Names,
		"pattern": n.gp.Pattern,
		"labels":  n.gp.Labels,
		"limit":   n.limit,
		"random":  random,
	}).Debug("listing matching containers")
	addCmdPrefix, delCmdPrefix, cmdSuffix := n.buildIPTablesCmd()
	return chaos.RunOnContainers(ctx, n.client, n.gp, n.limit, random, true,
		func(ctx context.Context, c *container.Container) error {
			log.WithFields(log.Fields{"container": *c}).Debug("adding network random packet loss for container")
			iptCtx, cancel := context.WithTimeout(ctx, n.req.Duration)
			defer cancel()
			addReq := *n.req
			addReq.Container = c
			addReq.CmdPrefix = addCmdPrefix
			addReq.CmdSuffix = cmdSuffix
			delReq := addReq
			delReq.CmdPrefix = delCmdPrefix
			if err := runIPTables(iptCtx, n.client, &addReq, &delReq); err != nil {
				log.WithError(err).Warn("failed to set packet loss for container")
				return fmt.Errorf("failed to add packet loss for one or more containers: %w", err)
			}
			return nil
		})
}

func (n *lossCommand) buildIPTablesCmd() (addCmdPrefix, delCmdPrefix, cmdSuffix []string) {
	cmdPrefix := []string{"INPUT", "-i", n.iface}
	if n.protocol != "any" {
		cmdPrefix = append(cmdPrefix, "-p", n.protocol)
	}
	addCmdPrefix = append([]string{"-I"}, cmdPrefix...)
	delCmdPrefix = append([]string{"-D"}, cmdPrefix...)
	cmdSuffix = []string{"-m", "statistic", "--mode", n.mode}
	if n.mode == ModeRandom {
		cmdSuffix = append(cmdSuffix, "--probability", strconv.FormatFloat(n.probability, 'f', 2, 64))
	} else { // mode == nth
		cmdSuffix = append(cmdSuffix, "--every", strconv.Itoa(n.every), "--packet", strconv.Itoa(n.packet))
	}
	cmdSuffix = append(cmdSuffix, "-j", "DROP")
	return addCmdPrefix, delCmdPrefix, cmdSuffix
}
