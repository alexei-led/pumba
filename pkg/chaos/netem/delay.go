package netem

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
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
	iface        string
	ips          []net.IP
	duration     time.Duration
	time         int
	jitter       int
	correlation  float64
	distribution string
	image        string
	dryRun       bool
}

// NewDelayCommand create new netem delay command
func NewDelayCommand(client container.Client,
	names []string, // containers
	pattern string, // re2 regex pattern
	iface string, // network interface
	ipsList []string, // list of target ips
	durationStr string, // chaos duration
	intervalStr string, // repeatable chaos interval
	time int, // delay time
	jitter int, // delay jitter
	correlation float64, // delay correlation
	distribution string, // delay distribution
	image string, // traffic control image
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
		iface:        iface,
		ips:          ips,
		duration:     duration,
		time:         time,
		jitter:       jitter,
		correlation:  correlation,
		distribution: distribution,
		image:        image,
		dryRun:       dryRun,
	}, nil
}

// Run netem delay command
func (s *DelayCommand) Run(ctx context.Context, random bool) error {
	return nil
}
