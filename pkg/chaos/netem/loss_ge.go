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

// LossGECommand `netem loss gemodel` (Gilbert-Elliot model) command
type LossGECommand struct {
	client   container.Client
	names    []string
	pattern  string
	iface    string
	ips      []net.IP
	duration time.Duration
	pg       float64
	pb       float64
	oneH     float64
	oneK     float64
	image    string
	limit    int
	dryRun   bool
}

// NewLossGECommand create new netem loss gemodel (Gilbert-Elliot) command
func NewLossGECommand(client container.Client,
	names []string, // containers
	pattern string, // re2 regex pattern
	iface string, // network interface
	ipsList []string, // list of target ips
	durationStr string, // chaos duration
	intervalStr string, // repeatable chaos interval
	pg float64, // Good State transition probability
	pb float64, // Bad State transition probability
	oneH float64, // loss probability in Bad state
	oneK float64, // loss probability in Good state
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
	// get pg - Good State transition probability
	if pg < 0.0 || pg > 100.0 {
		err = errors.New("Invalid pg (Good State) transition probability: must be between 0.0 and 100.0")
		log.Error(err)
		return nil, err
	}
	// get pb - Bad State transition probability
	if pb < 0.0 || pb > 100.0 {
		err = errors.New("Invalid pb (Bad State) transition probability: must be between 0.0 and 100.0")
		log.Error(err)
		return nil, err
	}
	// get (1-h) - loss probability in Bad state
	if oneH < 0.0 || oneH > 100.0 {
		err = errors.New("Invalid loss probability: must be between 0.0 and 100.0")
		log.Error(err)
		return nil, err
	}
	// get (1-k) - loss probability in Good state
	if oneK < 0.0 || oneK > 100.0 {
		err = errors.New("Invalid loss probability: must be between 0.0 and 100.0")
		log.Error(err)
		return nil, err
	}

	return &LossGECommand{
		client:   client,
		names:    names,
		pattern:  pattern,
		iface:    iface,
		ips:      ips,
		duration: duration,
		pg:       pg,
		pb:       pb,
		oneH:     oneH,
		oneK:     oneK,
		image:    image,
		limit:    limit,
		dryRun:   dryRun,
	}, nil
}

// Run netem loss state command
func (n *LossGECommand) Run(ctx context.Context, random bool) error {
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

	// prepare netem loss gemodel command
	netemCmd := []string{"loss", "gemodel", strconv.FormatFloat(n.pg, 'f', 2, 64)}
	netemCmd = append(netemCmd, strconv.FormatFloat(n.pb, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(n.oneH, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(n.oneK, 'f', 2, 64))

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
