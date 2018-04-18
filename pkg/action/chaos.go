package action

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"regexp"
	"strconv"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

var (
	// RandomMode - select random container from matching list
	RandomMode = false
	// DryMode - do not 'kill' the container only log event
	DryMode = false
	// DelayDistribution netem delay distributions
	DelayDistribution = []string{"", "uniform", "normal", "pareto", "paretonormal"}
)

// CommandNetemDelay arguments for 'netem delay' sub-command
type CommandNetemDelay struct {
	NetInterface string
	IPs          []net.IP
	Duration     time.Duration
	Time         int
	Jitter       int
	Correlation  float64
	Distribution string
	Image        string
}

// CommandNetemLossRandom arguments for 'netem loss' (random) sub-command
type CommandNetemLossRandom struct {
	NetInterface string
	IPs          []net.IP
	Duration     time.Duration
	Percent      float64
	Correlation  float64
	Image        string
}

// CommandNetemLossState arguments for 'netem loss state' sub-command
type CommandNetemLossState struct {
	NetInterface string
	IPs          []net.IP
	Duration     time.Duration
	P13          float64
	P31          float64
	P32          float64
	P23          float64
	P14          float64
	Image        string
}

// CommandNetemLossGEmodel arguments for 'netem loss gemodel' sub-command
type CommandNetemLossGEmodel struct {
	NetInterface string
	IPs          []net.IP
	Duration     time.Duration
	PG           float64
	PB           float64
	OneH         float64
	OneK         float64
	Image        string
}

// CommandNetemRate arguments for 'netem rate' sub-command
type CommandNetemRate struct {
	NetInterface   string
	IPs            []net.IP
	Duration       time.Duration
	Rate           string
	PacketOverhead int
	CellSize       int
	CellOverhead   int
	Image          string
}

// CommandRemove arguments for remove command
type CommandRemove struct {
	Force   bool
	Links   bool
	Volumes bool
}

// A Chaos is the interface with different methods to stop running containers.
type Chaos interface {
	RemoveContainers(context.Context, container.Client, []string, string, interface{}) error
	NetemDelayContainers(context.Context, container.Client, []string, string, interface{}) error
	NetemLossRandomContainers(context.Context, container.Client, []string, string, interface{}) error
	NetemLossStateContainers(context.Context, container.Client, []string, string, interface{}) error
	NetemLossGEmodelContainers(context.Context, container.Client, []string, string, interface{}) error
	NetemRateContainers(context.Context, container.Client, []string, string, interface{}) error
}

// NewChaos create new Pumba Chaos instance
func NewChaos() Chaos {
	return pumbaChaos{}
}

// pumba makes Chaos
type pumbaChaos struct {
}

// all containers beside Pumba and PumbaSkip
func allContainersFilter(c container.Container) bool {
	if c.IsPumba() || c.IsPumbaSkip() {
		return false
	}
	return true
}

func containerFilter(names []string) container.Filter {
	if len(names) == 0 {
		return allContainersFilter
	}

	return func(c container.Container) bool {
		if c.IsPumba() || c.IsPumbaSkip() {
			return false
		}
		for _, name := range names {
			if (name == c.Name()) || (name == c.Name()[1:]) {
				return true
			}
		}
		return false
	}
}

func regexContainerFilter(pattern string) container.Filter {
	return func(c container.Container) bool {
		if c.IsPumba() || c.IsPumbaSkip() {
			return false
		}
		matched, err := regexp.MatchString(pattern, c.Name())
		if err != nil {
			return false
		}
		// container name may start with forward slash, when using inspect function
		if !matched {
			matched, err = regexp.MatchString(pattern, c.Name()[1:])
			if err != nil {
				return false
			}
		}
		return matched
	}
}

func listRunningContainers(ctx context.Context, client container.Client, names []string, pattern string) ([]container.Container, error) {
	return listContainers(ctx, client, names, pattern, false)
}

func listAllContainers(ctx context.Context, client container.Client, names []string, pattern string) ([]container.Container, error) {
	return listContainers(ctx, client, names, pattern, true)
}

func listContainers(ctx context.Context, client container.Client, names []string, pattern string, all bool) ([]container.Container, error) {
	var err error
	var containers []container.Container
	var filter container.Filter

	if pattern != "" {
		filter = regexContainerFilter(pattern)
	} else {
		filter = containerFilter(names)
	}

	if all {
		containers, err = client.ListAllContainers(ctx, filter)
	} else {
		containers, err = client.ListContainers(ctx, filter)
	}

	if err != nil {
		return nil, err
	}

	return containers, nil

}

func randomContainer(containers []container.Container) *container.Container {
	if len(containers) > 0 {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		i := r.Intn(len(containers))
		log.Debug(i, "  ", containers[i])
		return &containers[i]
	}
	return nil
}

func removeContainers(ctx context.Context, client container.Client, containers []container.Container, force bool, links bool, volumes bool) error {
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			err := client.RemoveContainer(ctx, *container, force, links, volumes, DryMode)
			if err != nil {
				return err
			}
		}
	} else {
		for _, container := range containers {
			err := client.RemoveContainer(ctx, container, force, links, volumes, DryMode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func netemContainers(ctx context.Context, client container.Client, containers []container.Container, netInterface string, netemCmd []string, ips []net.IP, duration time.Duration, tcimage string) error {
	var err error
	netemContainers := []container.Container{}
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			err = client.NetemContainer(ctx, *container, netInterface, netemCmd, ips, duration, tcimage, DryMode)
			if err != nil {
				return err
			}
			netemContainers = append(netemContainers, *container)
		}
	} else {
		for _, container := range containers {
			err = client.NetemContainer(ctx, container, netInterface, netemCmd, ips, duration, tcimage, DryMode)
			if err != nil {
				break
			} else {
				netemContainers = append(netemContainers, container)
			}
		}
	}
	// wait for specified duration and then stop netem (where it applied) or stop on ctx.Done()
	select {
	case <-ctx.Done():
		log.Debugf("Stopping netem by stop event")
		// use different context to stop netem since parent context is canceled
		err = stopNetemContainers(context.Background(), client, netemContainers, netInterface, ips, tcimage)
	case <-time.After(duration):
		log.Debugf("Stopping netem after: %s", duration)
		err = stopNetemContainers(ctx, client, netemContainers, netInterface, ips, tcimage)
	}

	return err
}

func stopNetemContainers(ctx context.Context, client container.Client, containers []container.Container, netInterface string, ips []net.IP, tcimage string) error {
	var err error
	for _, container := range containers {
		if e := client.StopNetemContainer(ctx, container, netInterface, ips, tcimage, DryMode); e != nil {
			err = e
		}
	}
	return err // last non nil error
}

//---------------------------------------------------------------------------------------------------

// RemoveContainers - remove container either by RE2 pattern (if specified) or by names
func (p pumbaChaos) RemoveContainers(ctx context.Context, client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("Remove containers")
	// get command details
	command, ok := cmd.(CommandRemove)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandRemove")
	}
	var err error
	var containers []container.Container
	if containers, err = listRunningContainers(ctx, client, names, pattern); err != nil {
		return err
	}
	return removeContainers(ctx, client, containers, command.Force, command.Links, command.Volumes)
}

// NetemDelayContainers delay network traffic with optional Jitter and correlation
func (p pumbaChaos) NetemDelayContainers(ctx context.Context, client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("netem: delay for containers")
	// get command details
	command, ok := cmd.(CommandNetemDelay)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandNetemDelay")
	}
	var err error
	var containers []container.Container
	if containers, err = listRunningContainers(ctx, client, names, pattern); err != nil {
		return err
	}
	netemCmd := []string{"delay", strconv.Itoa(command.Time) + "ms"}
	if command.Jitter > 0 {
		netemCmd = append(netemCmd, strconv.Itoa(command.Jitter)+"ms")
	}
	if command.Correlation > 0 {
		netemCmd = append(netemCmd, strconv.FormatFloat(command.Correlation, 'f', 2, 64))
	}
	if command.Distribution != "" {
		netemCmd = append(netemCmd, []string{"distribution", command.Distribution}...)
	}

	return netemContainers(ctx, client, containers, command.NetInterface, netemCmd, command.IPs, command.Duration, command.Image)
}

func (p pumbaChaos) NetemLossRandomContainers(ctx context.Context, client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("netem: loss random for containers")
	// get command details
	command, ok := cmd.(CommandNetemLossRandom)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandNetemLossRandom")
	}
	var err error
	var containers []container.Container
	if containers, err = listRunningContainers(ctx, client, names, pattern); err != nil {
		return err
	}
	// prepare netem loss command
	netemCmd := []string{"loss", strconv.FormatFloat(command.Percent, 'f', 2, 64)}
	if command.Correlation > 0 {
		netemCmd = append(netemCmd, strconv.FormatFloat(command.Correlation, 'f', 2, 64))
	}
	// run prepared netem loss command
	return netemContainers(ctx, client, containers, command.NetInterface, netemCmd, command.IPs, command.Duration, command.Image)
}

func (p pumbaChaos) NetemLossStateContainers(ctx context.Context, client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("netem: loss random for containers")
	// get command details
	command, ok := cmd.(CommandNetemLossState)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandNetemLossState")
	}
	var err error
	var containers []container.Container
	if containers, err = listRunningContainers(ctx, client, names, pattern); err != nil {
		return err
	}
	// prepare netem loss state command
	netemCmd := []string{"loss", "state", strconv.FormatFloat(command.P13, 'f', 2, 64)}
	netemCmd = append(netemCmd, strconv.FormatFloat(command.P31, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(command.P32, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(command.P23, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(command.P14, 'f', 2, 64))
	// run prepared netem state command
	return netemContainers(ctx, client, containers, command.NetInterface, netemCmd, command.IPs, command.Duration, command.Image)
}

func (p pumbaChaos) NetemLossGEmodelContainers(ctx context.Context, client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("netem: loss random for containers")
	// get command details
	command, ok := cmd.(CommandNetemLossGEmodel)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandNetemLossGEmodel")
	}
	var err error
	var containers []container.Container
	if containers, err = listRunningContainers(ctx, client, names, pattern); err != nil {
		return err
	}
	// prepare netem loss gemodel command
	netemCmd := []string{"loss", "gemodel", strconv.FormatFloat(command.PG, 'f', 2, 64)}
	netemCmd = append(netemCmd, strconv.FormatFloat(command.PB, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(command.OneH, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(command.OneK, 'f', 2, 64))
	// run prepared netem loss gemodel command
	return netemContainers(ctx, client, containers, command.NetInterface, netemCmd, command.IPs, command.Duration, command.Image)
}

// NetemRateContainers delay network traffic with optional Jitter and correlation
func (p pumbaChaos) NetemRateContainers(ctx context.Context, client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("netem: rate for containers")
	// get command details
	command, ok := cmd.(CommandNetemRate)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandNetemRate")
	}
	var err error
	var containers []container.Container
	if containers, err = listRunningContainers(ctx, client, names, pattern); err != nil {
		return err
	}
	netemCmd := []string{"rate", command.Rate}
	if command.PacketOverhead != 0 {
		netemCmd = append(netemCmd, strconv.Itoa(command.PacketOverhead))
	}
	if command.CellSize > 0 {
		netemCmd = append(netemCmd, strconv.Itoa(command.CellSize))
	}
	if command.CellOverhead != 0 {
		netemCmd = append(netemCmd, strconv.Itoa(command.CellOverhead))
	}

	return netemContainers(ctx, client, containers, command.NetInterface, netemCmd, command.IPs, command.Duration, command.Image)
}
