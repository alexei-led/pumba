package iptables

import (
	"context"
	"strconv"
	"sync"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// `iptables loss` command
type lossCommand struct {
	ipTablesCommand
	mode        string
	probability float64
	every       int
	packet      int
}

// NewLossCommand create new iptables loss command
func NewLossCommand(client container.Client,
	globalParams *chaos.GlobalParams,
	params *Params,
	mode string, // loss mode
	probability float64, // loss probability
	every int, // drop every nth
	packet int, // start budget for every nth
) (chaos.Command, error) {
	// get mode
	if mode == "random" {
		// get loss probability
		if probability < 0.0 || probability > 1.0 {
			return nil, errors.Errorf("invalid loss probability: must be between 0.0 and 1.0")
		}
	} else if mode == "nth" {
		// get every
		if every <= 0 {
			return nil, errors.Errorf("invalid loss every: must be > 0")
		}
		// get packet
		if packet < 0 || (packet > every-1) {
			return nil, errors.Errorf("invalid loss packet: must be 0 <= packet <= every-1")
		}
	} else {
		return nil, errors.Errorf("invalid loss mode: must be either random or nth")
	}

	return &lossCommand{
		ipTablesCommand: newIPTablesCommand(client, globalParams, params),
		mode:            mode,
		probability:     probability,
		every:           every,
		packet:          packet,
	}, nil
}

// Run iptables loss command
func (n *lossCommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network random packet loss to all matching containers")
	log.WithFields(log.Fields{
		"names":   n.names,
		"pattern": n.pattern,
		"labels":  n.labels,
		"limit":   n.limit,
		"random":  random,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, n.client, n.names, n.pattern, n.labels, n.limit)
	if err != nil {
		return errors.Wrap(err, "error listing containers")
	}
	if len(containers) == 0 {
		log.Warning("no containers found")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		if c := container.RandomContainer(containers); c != nil {
			containers = []*container.Container{c}
		}
	}

	// prepare iptables command prefix
	cmdPrefix := []string{"INPUT", "-i", n.iface}
	if n.protocol != "any" {
		cmdPrefix = append(cmdPrefix, "-p", n.protocol)
	}

	// prepare iptables add command prefix
	addCmdPrefix := []string{"-I"}
	addCmdPrefix = append(addCmdPrefix, cmdPrefix...)

	// prepare iptables del command prefix
	delCmdPrefix := []string{"-D"}
	delCmdPrefix = append(delCmdPrefix, cmdPrefix...)

	// prepare iptables loss command suffix
	cmdSuffix := []string{"-m", "statistic", "--mode", n.mode}
	if n.mode == "random" {
		cmdSuffix = append(cmdSuffix, "--probability", strconv.FormatFloat(n.probability, 'f', 2, 64))
	} else { // mode == nth
		cmdSuffix = append(cmdSuffix, "--every", strconv.Itoa(n.every), "--packet", strconv.Itoa(n.packet))
	}
	cmdSuffix = append(cmdSuffix, "-j", "DROP")

	// run iptables loss command for selected containers
	var wg sync.WaitGroup
	errs := make([]error, len(containers))
	cancels := make([]context.CancelFunc, len(containers))
	for i, c := range containers {
		log.WithFields(log.Fields{
			"container": *c,
		}).Debug("adding network random packet loss for container")
		ctx, cancel := context.WithTimeout(ctx, n.duration)
		cancels[i] = cancel
		wg.Add(1)
		go func(i int, c *container.Container) {
			defer wg.Done()
			errs[i] = runIPTables(ctx, n.client, c, addCmdPrefix, delCmdPrefix, cmdSuffix, n.srcIPs, n.dstIPs, n.sports, n.dports, n.duration, n.image, n.pull, n.dryRun)
			if errs[i] != nil {
				log.WithError(errs[i]).Warn("failed to set packet loss for container")
			}
		}(i, c)
	}

	// Wait for all iptables delay commands to complete
	wg.Wait()

	// cancel context to avoid leaks
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	// scan through all errors in goroutines
	for _, err = range errs {
		// take first found error
		if err != nil {
			return errors.Wrap(err, "failed to add packet loss for one or more containers")
		}
	}

	return nil
}
