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

var (
	// DelayDistribution netem delay distributions
	delayDistribution = []string{"", "uniform", "normal", "pareto", "paretonormal"}
)

// DelayCommand `netem delay` command
type DelayCommand struct {
	client       container.Client
	names        []string
	pattern      string
	labels       []string
	iface        string
	ips          []*net.IPNet
	sports       []string
	dports       []string
	duration     time.Duration
	time         int
	jitter       int
	correlation  float64
	distribution string
	image        string
	pull         bool
	limit        int
	dryRun       bool
}

// NewDelayCommand create new netem delay command
func NewDelayCommand(client container.Client,
	names []string, // containers
	pattern string, // re2 regex pattern
	labels []string, // filter by labels
	iface string, // network interface
	ipsList []string, // list of target ips
	sportsList, // list of comma separated target sports
	dportsList, // list of comma separated target dports
	durationStr, // chaos duration
	intervalStr string, // repeatable chaos interval
	delay, // delay time
	jitter int, // delay jitter
	correlation float64, // delay correlation
	distribution, // delay distribution
	image string, // traffic control image
	pull bool, // pull tc image option
	limit int, // limit chaos to containers
	dryRun bool, // dry-run do not delay just log
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
	reInterface := regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9.:_-]*`)
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
	// check delay time
	if delay <= 0 {
		return nil, errors.New("non-positive delay time")
	}
	// get delay variation
	if jitter < 0 || jitter > delay {
		return nil, errors.New("invalid delay jitter: must be non-negative and smaller than delay time")
	}
	// get delay variation
	if correlation < 0.0 || correlation > 100.0 {
		return nil, errors.New("invalid delay correlation: must be between 0.0 and 100.0")
	}
	// get distribution
	if ok := util.SliceContains(delayDistribution, distribution); !ok {
		return nil, errors.New("invalid delay distribution: must be one of {uniform | normal | pareto |  paretonormal}")
	}

	return &DelayCommand{
		client:       client,
		names:        names,
		pattern:      pattern,
		labels:       labels,
		iface:        iface,
		ips:          ips,
		sports:       sports,
		dports:       dports,
		duration:     duration,
		time:         delay,
		jitter:       jitter,
		correlation:  correlation,
		distribution: distribution,
		image:        image,
		pull:         pull,
		limit:        limit,
		dryRun:       dryRun,
	}, nil
}

// Run netem delay command
func (n *DelayCommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network delay to all matching containers")
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

	// prepare netem command
	netemCmd := []string{"delay", strconv.Itoa(n.time) + "ms"}
	if n.jitter > 0 {
		netemCmd = append(netemCmd, strconv.Itoa(n.jitter)+"ms")
	}
	if n.correlation > 0 {
		netemCmd = append(netemCmd, strconv.FormatFloat(n.correlation, 'f', 2, 64))
	}
	if n.distribution != "" {
		netemCmd = append(netemCmd, []string{"distribution", n.distribution}...)
	}

	// run netem delay command for selected containers
	var wg sync.WaitGroup
	errs := make([]error, len(containers))
	cancels := make([]context.CancelFunc, len(containers))
	for i, c := range containers {
		log.WithFields(log.Fields{
			"container": c,
		}).Debug("adding network delay for container")
		netemCtx, cancel := context.WithCancel(ctx)
		cancels[i] = cancel
		wg.Add(1)
		go func(i int, c *container.Container) {
			defer wg.Done()
			errs[i] = runNetem(netemCtx, n.client, c, n.iface, netemCmd, n.ips, n.sports, n.dports, n.duration, n.image, n.pull, n.dryRun)
			if errs[i] != nil {
				log.WithError(errs[i]).Warn("failed to delay network for container")
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
			return errors.Wrap(err, "failed to delay packets for one or more containers")
		}
	}

	return nil
}
