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

// CorruptCommand `netem corrupt` command
type CorruptCommand struct {
	client      container.Client
	names       []string
	pattern     string
	labels      []string
	iface       string
	ips         []*net.IPNet
	sports      []string
	dports      []string
	duration    time.Duration
	percent     float64
	correlation float64
	image       string
	pull        bool
	limit       int
	dryRun      bool
}

// NewCorruptCommand create new netem corrupt command
func NewCorruptCommand(client container.Client,
	names []string, // containers
	pattern string, // re2 regex pattern
	labels []string, // filter by labels
	iface string, // network interface
	ipsList []string, // list of target ips
	sportsList, // list of comma separated target sports
	dportsList, // list of comma separated target dports
	durationStr, // chaos duration
	intervalStr string, // repeatable chaos interval
	percent, // corrupt percent
	correlation float64, // corrupt correlation
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
	reInterface := regexp.MustCompile("[a-zA-Z][a-zA-Z0-9\.:_-]*")
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
	// get netem corrupt percent
	if percent < 0.0 || percent > 100.0 {
		return nil, errors.New("invalid corrupt percent: must be between 0.0 and 100.0")
	}
	// get netem corrupt variation
	if correlation < 0.0 || correlation > 100.0 {
		return nil, errors.New("invalid corrupt correlation: must be between 0.0 and 100.0")
	}

	return &CorruptCommand{
		client:      client,
		names:       names,
		labels:      labels,
		pattern:     pattern,
		iface:       iface,
		ips:         ips,
		sports:      sports,
		dports:      dports,
		duration:    duration,
		percent:     percent,
		correlation: correlation,
		image:       image,
		pull:        pull,
		limit:       limit,
		dryRun:      dryRun,
	}, nil
}

// Run netem corrupt command
func (n *CorruptCommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network random packet corrupt to all matching containers")
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

	// prepare netem corrupt command
	netemCmd := []string{"corrupt", strconv.FormatFloat(n.percent, 'f', 2, 64)}
	if n.correlation > 0 {
		netemCmd = append(netemCmd, strconv.FormatFloat(n.correlation, 'f', 2, 64))
	}

	// run netem corrupt command for selected containers
	var wg sync.WaitGroup
	errs := make([]error, len(containers))
	cancels := make([]context.CancelFunc, len(containers))
	for i, c := range containers {
		log.WithFields(log.Fields{
			"container": c,
		}).Debug("adding network random packet corrupt for container")
		netemCtx, cancel := context.WithTimeout(ctx, n.duration)
		cancels[i] = cancel
		wg.Add(1)
		go func(i int, c *container.Container) {
			defer wg.Done()
			errs[i] = runNetem(netemCtx, n.client, c, n.iface, netemCmd, n.ips, n.sports, n.dports, n.duration, n.image, n.pull, n.dryRun)
			if errs[i] != nil {
				log.WithError(errs[i]).Warn("failed to set packet corrupt for container")
			}
		}(i, c)
	}

	// Wait for all netem commands to complete
	wg.Wait()

	// cancel context to avoid leaks
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	// scan through all errors in goroutines
	for _, err = range errs {
		// take first reported error
		if err != nil {
			return errors.Wrap(err, "failed to corrupt packets for one or more containers")
		}
	}

	return nil
}
