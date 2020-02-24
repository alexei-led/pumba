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
	durationStr string, // chaos duration
	intervalStr string, // repeatable chaos interval
	time int, // delay time
	jitter int, // delay jitter
	correlation float64, // delay correlation
	distribution string, // delay distribution
	image string, // traffic control image
	pull bool, // pull tc image option
	limit int, // limit chaos to containers
	dryRun bool, // dry-run do not delay just log
) (chaos.Command, error) {
	// log error
	var err error
	defer func() {
		if err != nil {
			log.WithError(err).Error("failed to construct Netem Delay Command")
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
	var ips []*net.IPNet
	for _, str := range ipsList {
		ip := util.ParseCIDR(str)
		if ip == nil {
			err = fmt.Errorf("bad target: '%s' is not a valid IP", str)
			return nil, err
		}
		ips = append(ips, ip)
	}
	// check delay time
	if time <= 0 {
		err = errors.New("non-positive delay time")
		return nil, err
	}
	// get delay variation
	if jitter < 0 || jitter > time {
		err = errors.New("invalid delay jitter: must be non-negative and smaller than delay time")
		return nil, err
	}
	// get delay variation
	if correlation < 0.0 || correlation > 100.0 {
		err = errors.New("invalid delay correlation: must be between 0.0 and 100.0")
		return nil, err
	}
	// get distribution
	if ok := util.SliceContains(delayDistribution, distribution); !ok {
		err = errors.New("Invalid delay distribution: must be one of {uniform | normal | pareto |  paretonormal}")
		return nil, err
	}

	return &DelayCommand{
		client:       client,
		names:        names,
		pattern:      pattern,
		labels:       labels,
		iface:        iface,
		ips:          ips,
		duration:     duration,
		time:         time,
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
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, n.client, n.names, n.pattern, n.labels, n.limit)
	if err != nil {
		log.WithError(err).Error("failed to list containers")
		return err
	}
	if len(containers) == 0 {
		err := fmt.Errorf("no containers matching names = %s, pattern = %s, labels = %s", n.names, n.pattern, n.labels)
		log.WithError(err).Error("no containers to netem delay")
		return err
	}

	// select single random container from matching container and replace list with selected item
	if random {
		log.Debug("selecting single random container")
		if c := container.RandomContainer(containers); c != nil {
			containers = []container.Container{*c}
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
	errors := make([]error, len(containers))
	cancels := make([]context.CancelFunc, len(containers))
	for i, c := range containers {
		log.WithFields(log.Fields{
			"container": c,
		}).Debug("adding network delay for container")
		netemCtx, cancel := context.WithCancel(ctx)
		cancels[i] = cancel
		wg.Add(1)
		go func(i int, c container.Container) {
			defer wg.Done()
			errors[i] = runNetem(netemCtx, n.client, c, n.iface, netemCmd, n.ips, n.duration, n.image, n.pull, n.dryRun)
			if errors[i] != nil {
				log.WithField("container", c).WithError(errors[i]).Error("failed to delay network for container")
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
