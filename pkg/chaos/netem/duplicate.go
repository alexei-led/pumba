package netem

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/util"

	log "github.com/sirupsen/logrus"
)

// DuplicateCommand `netem duplicate` command
type DuplicateCommand struct {
	client      container.Client
	names       []string
	pattern     string
	iface       string
	ips         []net.IP
	duration    time.Duration
	percent     float64
	correlation float64
	image       string
	limit       int
	dryRun      bool
}

// NewDuplicateCommand create new netem duplicate command
func NewDuplicateCommand(client container.Client,
	names []string, // containers
	pattern string, // re2 regex pattern
	iface string, // network interface
	ipsList []string, // list of target ips
	durationStr string, // chaos duration
	intervalStr string, // repeatable chaos interval
	percent float64, // duplicate percent
	correlation float64, // duplicate correlation
	image string, // traffic control image
	limit int, // limit chaos to containers
	dryRun bool, // dry-run do not netem just log
) (chaos.Command, error) {
	// log error
	var err error
	defer func() {
		if err != nil {
			log.WithError(err).Error("failed to construct Netem Duplicate Command")
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
	reInterface := regexp.MustCompile("[a-zA-Z]+[0-9]{0,2}")
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
	// get netem duplicate percent
	if percent < 0.0 || percent > 100.0 {
		err = errors.New("invalid duplicate percent: must be between 0.0 and 100.0")
		return nil, err
	}
	// get netem duplicate variation
	if correlation < 0.0 || correlation > 100.0 {
		err = errors.New("invalid duplicate correlation: must be between 0.0 and 100.0")
		return nil, err
	}

	return &DuplicateCommand{
		client:      client,
		names:       names,
		pattern:     pattern,
		iface:       iface,
		ips:         ips,
		duration:    duration,
		percent:     percent,
		correlation: correlation,
		image:       image,
		limit:       limit,
		dryRun:      dryRun,
	}, nil
}

// Run netem duplicate command
func (n *DuplicateCommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network random packet duplicates to all matching containers")
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

	// prepare netem duplicate command
	netemCmd := []string{"duplicate", strconv.FormatFloat(n.percent, 'f', 2, 64)}
	if n.correlation > 0 {
		netemCmd = append(netemCmd, strconv.FormatFloat(n.correlation, 'f', 2, 64))
	}

	// run netem duplicate command for selected containers
	var cancels []context.CancelFunc
	for _, c := range containers {
		log.WithFields(log.Fields{
			"container": c,
		}).Debug("adding network random packet duplicates for container")
		netemCtx, cancel := context.WithTimeout(ctx, n.duration)
		cancels = append(cancels, cancel)
		err := runNetem(netemCtx, n.client, c, n.iface, netemCmd, n.ips, n.duration, n.image, n.dryRun)
		if err != nil {
			log.WithError(err).Error("failed to set packet duplicates for container")
			return err
		}
	}

	// cancel context to avoid leaks
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	return nil
}
