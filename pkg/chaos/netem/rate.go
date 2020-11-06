package netem

import (
	"context"
	"net"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/util"
	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
)

// Parse rate
func parseRate(rate string) (string, error) {
	reRate := regexp.MustCompile(`\d+[gmk]?bit`)
	validRate := reRate.FindString(rate)
	if rate != validRate {
		return "", errors.Errorf("invalid rate, must match '%s'", reRate.String())
	}
	return rate, nil
}

// RateCommand `netem rate` command
type RateCommand struct {
	client         container.Client
	names          []string
	pattern        string
	labels         []string
	iface          string
	ips            []*net.IPNet
	sports         []string
	dports         []string
	duration       time.Duration
	rate           string
	packetOverhead int
	cellSize       int
	cellOverhead   int
	image          string
	pull           bool
	limit          int
	dryRun         bool
}

// NewRateCommand create new netem rate command
func NewRateCommand(client container.Client,
	names []string, // containers
	pattern string, // re2 regex pattern
	labels []string, // filter by labels
	iface string, // network interface
	ipsList []string, // list of target ips
	sportsList, // list of comma separated target sports
	dportsList, // list of comma separated target dports
	durationStr, // chaos duration
	intervalStr, // repeatable chaos interval
	rate string, // delay outgoing packets; in common units
	packetOverhead, // per packet overhead; in bytes
	cellSize, // cell size of the simulated link layer scheme
	cellOverhead int, // per cell overhead; in bytes
	image string, // traffic control image
	pull bool, // pull tc image
	limit int, // limit chaos to containers
	dryRun bool, // dry-run do not netem just log
) (chaos.Command, error) {
	// get interval
	interval, err := util.GetIntervalValue(intervalStr)
	if err != nil {
		return nil, err
	}
	// get duration
	duration, err := util.GetDurationValue(durationStr, interval)
	if err != nil {
		return nil, err
	}
	// protect from Command Injection, using Regexp
	reInterface := regexp.MustCompile("[a-zA-Z][a-zA-Z0-9\\.:_-]*")
	validIface := reInterface.FindString(iface)
	if iface != validIface {
		return nil, errors.Errorf("bad network interface name: must match '%s'", reInterface.String())
	}
	// validate ips
	var ips []*net.IPNet
	for _, str := range ipsList {
		ip, e := util.ParseCIDR(str)
		if e != nil {
			return nil, e
		}
		ips = append(ips, ip)
	}
	// validate sports
	sports, err := util.GetPorts(sportsList)
	if err != nil {
		return nil, err
	}
	// validate dports
	dports, err := util.GetPorts(dportsList)
	if err != nil {
		return nil, err
	}
	// validate target egress rate
	if rate == "" {
		return nil, errors.New("undefined rate limit")
	}
	rate, err = parseRate(rate)
	if err != nil {
		return nil, err
	}

	// validate cell size
	if cellSize < 0 {
		return nil, errors.New("invalid cell size: must be a non-negative integer")
	}

	return &RateCommand{
		client:         client,
		names:          names,
		pattern:        pattern,
		labels:         labels,
		iface:          iface,
		ips:            ips,
		sports:         sports,
		dports:         dports,
		duration:       duration,
		rate:           rate,
		packetOverhead: packetOverhead,
		cellSize:       cellSize,
		cellOverhead:   cellOverhead,
		image:          image,
		pull:           pull,
		limit:          limit,
		dryRun:         dryRun,
	}, nil
}

// Run netem rate command
func (n *RateCommand) Run(ctx context.Context, random bool) error {
	log.Debug("setting network rate to all matching containers")
	log.WithFields(log.Fields{
		"names":   n.names,
		"pattern": n.pattern,
		"labels":  n.labels,
		"limit":   n.limit,
		"random":  random,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, n.client, n.names, n.pattern, n.labels, n.limit)
	if err != nil {
		return err
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

	// prepare netem rate command
	netemCmd := []string{"rate", n.rate}
	if n.packetOverhead != 0 {
		netemCmd = append(netemCmd, strconv.Itoa(n.packetOverhead))
	}
	if n.cellSize > 0 {
		netemCmd = append(netemCmd, strconv.Itoa(n.cellSize))
	}
	if n.cellOverhead != 0 {
		netemCmd = append(netemCmd, strconv.Itoa(n.cellOverhead))
	}

	// run netem loss command for selected containers
	var wg sync.WaitGroup
	errs := make([]error, len(containers))
	cancels := make([]context.CancelFunc, len(containers))
	for i, c := range containers {
		log.WithFields(log.Fields{
			"container": c,
			"command":   netemCmd,
		}).Debug("setting network rate for container")
		netemCtx, cancel := context.WithTimeout(ctx, n.duration)
		cancels[i] = cancel
		wg.Add(1)
		go func(i int, c *container.Container) {
			defer wg.Done()
			errs[i] = runNetem(netemCtx, n.client, c, n.iface, netemCmd, n.ips, n.sports, n.dports, n.duration, n.image, n.pull, n.dryRun)
			if errs[i] != nil {
				log.WithError(errs[i]).Warn("failed to set network rate for container")
			}
		}(i, c)
	}

	// Wait for all netem delay commands to complete
	wg.Wait()

	// cancel context to avoid leaks
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	// scan through all errors in goroutines
	for _, err := range errs {
		// take first found error
		if err != nil {
			return errors.Wrap(err, "failed to set network rate for one or more containers")
		}
	}

	return nil
}
