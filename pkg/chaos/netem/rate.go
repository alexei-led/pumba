package netem

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/util"

	log "github.com/sirupsen/logrus"
)

// Parse rate
func parseRate(rate string) (string, error) {
	reRate := regexp.MustCompile("[0-9]+[gmk]?bit")
	validRate := reRate.FindString(rate)
	if rate != validRate {
		err := fmt.Errorf("Invalid rate. Must match '%s'", reRate.String())
		log.Error(err)
		return "", err
	}
	return rate, nil
}

// RateCommand `netem rate` command
type RateCommand struct {
	client         container.Client
	names          []string
	pattern        string
	iface          string
	ips            []net.IP
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
	iface string, // network interface
	ipsList []string, // list of target ips
	durationStr string, // chaos duration
	intervalStr string, // repeatable chaos interval
	rate string, // delay outgoing packets; in common units
	packetOverhead int, // per packet overhead; in bytes
	cellSize int, // cell size of the simulated link layer scheme
	cellOverhead int, // per cell overhead; in bytes
	image string, // traffic control image
	pull bool, // pull tc image
	limit int, // limit chaos to containers
	dryRun bool, // dry-run do not netem just log
) (chaos.Command, error) {
	// log error
	var err error
	defer func() {
		if err != nil {
			log.WithError(err).Error("failed to construct Netem Rate Command")
		}
	}()

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
	reInterface := regexp.MustCompile("[a-zA-Z][a-zA-Z0-9_-]*")
	validIface := reInterface.FindString(iface)
	if iface != validIface {
		err = fmt.Errorf("bad network interface name: must match '%s'", reInterface.String())
		return nil, err
	}
	// validate ips
	var ips []net.IP
	for _, str := range ipsList {
		ip := net.ParseIP(str)
		if ip == nil {
			err = fmt.Errorf("bad target: '%s' is not a valid IP", str)
			return nil, err
		}
		ips = append(ips, ip)
	}
	// validate target egress rate
	if rate == "" {
		err = errors.New("Undefined rate limit")
		log.Error(err)
		return nil, err
	}
	rate, err = parseRate(rate)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	// validate cell size
	if cellSize < 0 {
		err = errors.New("Invalid cell size: must be a non-negative integer")
		log.Error(err)
		return nil, err
	}

	return &RateCommand{
		client:         client,
		names:          names,
		pattern:        pattern,
		iface:          iface,
		ips:            ips,
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
		"limit":   n.limit,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, n.client, n.names, n.pattern, n.limit)
	if err != nil {
		log.WithError(err).Error("failed to list containers")
		return err
	}
	if len(containers) == 0 {
		log.Warning("no containers found")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		log.Debug("selecting single random container")
		if c := container.RandomContainer(containers); c != nil {
			containers = []container.Container{*c}
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
	errors := make([]error, len(containers))
	cancels := make([]context.CancelFunc, len(containers))
	for i, c := range containers {
		log.WithFields(log.Fields{
			"container": c,
			"command":   netemCmd,
		}).Debug("setting network rate for container")
		netemCtx, cancel := context.WithTimeout(ctx, n.duration)
		cancels[i] = cancel
		wg.Add(1)
		go func(i int, c container.Container) {
			defer wg.Done()
			errors[i] = runNetem(netemCtx, n.client, c, n.iface, netemCmd, n.ips, n.duration, n.image, n.pull, n.dryRun)
			if errors[i] != nil {
				log.WithError(errors[i]).Error("failed to set network rate for container")
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
	for _, e := range errors {
		// take first found error
		if e != nil {
			err = e
			break
		}
	}

	return err
}
