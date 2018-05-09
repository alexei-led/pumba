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

// LossStateCommand `netem loss state` command
type LossStateCommand struct {
	client   container.Client
	names    []string
	pattern  string
	iface    string
	ips      []net.IP
	duration time.Duration
	p13      float64
	p31      float64
	p32      float64
	p23      float64
	p14      float64
	image    string
	limit    int
	dryRun   bool
}

// NewLossStateCommand create new netem loss state command
func NewLossStateCommand(client container.Client,
	names []string, // containers
	pattern string, // re2 regex pattern
	iface string, // network interface
	ipsList []string, // list of target ips
	durationStr string, // chaos duration
	intervalStr string, // repeatable chaos interval
	p13 float64, // probability to go from state (1) to state (3)
	p31 float64, // probability to go from state (3) to state (1)
	p32 float64, // probability to go from state (3) to state (2)
	p23 float64, // probability to go from state (2) to state (3)
	p14 float64, // probability to go from state (1) to state (4)
	image string, // traffic control image
	limit int, // limit chaos to containers
	dryRun bool, // dry-run do not netem just log
) (chaos.Command, error) {
	// log error
	var err error
	defer func() {
		if err != nil {
			log.WithError(err).Error("failed to construct Netem Loss Command")
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

	// validate p13
	if p13 < 0.0 || p13 > 100.0 {
		err = errors.New("Invalid p13 percentage: : must be between 0.0 and 100.0")
		log.Error(err)
		return nil, err
	}
	// validate p31
	if p31 < 0.0 || p31 > 100.0 {
		err = errors.New("Invalid p31 percentage: : must be between 0.0 and 100.0")
		log.Error(err)
		return nil, err
	}
	// validate p32
	if p32 < 0.0 || p32 > 100.0 {
		err = errors.New("Invalid p32 percentage: : must be between 0.0 and 100.0")
		log.Error(err)
		return nil, err
	}
	// vaidate p23
	if p23 < 0.0 || p23 > 100.0 {
		err = errors.New("Invalid p23 percentage: : must be between 0.0 and 100.0")
		log.Error(err)
		return nil, err
	}
	// validate p14
	if p14 < 0.0 || p14 > 100.0 {
		err = errors.New("Invalid p14 percentage: : must be between 0.0 and 100.0")
		log.Error(err)
		return nil, err
	}

	return &LossStateCommand{
		client:   client,
		names:    names,
		pattern:  pattern,
		iface:    iface,
		ips:      ips,
		duration: duration,
		p13:      p13,
		p31:      p31,
		p32:      p32,
		p23:      p23,
		p14:      p14,
		image:    image,
		limit:    limit,
		dryRun:   dryRun,
	}, nil
}

// Run netem loss state command
func (n *LossStateCommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network packet loss according 4-state Markov model to all matching containers")
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
		log.Warning("no containers to found")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		log.Debug("selecting single random container")
		if c := container.RandomContainer(containers); c != nil {
			containers = []container.Container{*c}
		}
	}

	// prepare netem loss state command
	netemCmd := []string{"loss", "state", strconv.FormatFloat(n.p13, 'f', 2, 64)}
	netemCmd = append(netemCmd, strconv.FormatFloat(n.p31, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(n.p32, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(n.p23, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(n.p14, 'f', 2, 64))

	// run netem loss command for selected containers
	var cancels []context.CancelFunc
	for _, c := range containers {
		log.WithFields(log.Fields{
			"container": c,
		}).Debug("adding network random packet loss for container")
		netemCtx, cancel := context.WithTimeout(ctx, n.duration)
		cancels = append(cancels, cancel)
		err := runNetem(netemCtx, n.client, c, n.iface, netemCmd, n.ips, n.duration, n.image, n.dryRun)
		if err != nil {
			log.WithError(err).Error("failed to delay network for container")
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
