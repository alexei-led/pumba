package action

import (
	"errors"
	"math/rand"
	"net"
	"regexp"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gaia-adm/pumba/container"
)

var (
	// RandomMode - select random container from matching list
	RandomMode = false
	// DryMode - do not 'kill' the container only log event
	DryMode = false
	// DelayDistribution netem dalay distributions
	DelayDistribution = []string{"", "uniform", "normal", "pareto", "paretonormal"}
)

const (
	// DeafultWaitTime time to wait before stopping container (in seconds)
	DeafultWaitTime = 10
	// DefaultKillSignal default kill signal
	DefaultKillSignal = "SIGKILL"
)

// CommandKill arguments for kill command
type CommandKill struct {
	Signal string
}

// CommandPause arguments for pause command
type CommandPause struct {
	Duration time.Duration
	StopChan <-chan bool
}

// CommandNetemDelay arguments for 'netem delay' sub-command
type CommandNetemDelay struct {
	NetInterface string
	IP           net.IP
	Duration     time.Duration
	Time         int
	Jitter       int
	Correlation  float64
	Distribution string
	StopChan     <-chan bool
	Image        string
}

// CommandNetemLossRandom arguments for 'netem loss' (random) sub-command
type CommandNetemLossRandom struct {
	NetInterface string
	IP           net.IP
	Duration     time.Duration
	Percent      float64
	Correlation  float64
	StopChan     <-chan bool
	Image        string
}

// CommandNetemLossState arguments for 'netem loss state' sub-command
type CommandNetemLossState struct {
	NetInterface string
	IP           net.IP
	Duration     time.Duration
	P13          float64
	P31          float64
	P32          float64
	P23          float64
	P14          float64
	StopChan     <-chan bool
	Image        string
}

// CommandNetemLossGEmodel arguments for 'netem loss gemodel' sub-command
type CommandNetemLossGEmodel struct {
	NetInterface string
	IP           net.IP
	Duration     time.Duration
	PG           float64
	PB           float64
	OneH         float64
	OneK         float64
	StopChan     <-chan bool
	Image        string
}

// CommandStop arguments for stop command
type CommandStop struct {
	WaitTime int
}

// CommandRemove arguments for remove command
type CommandRemove struct {
	Force   bool
	Links   bool
	Volumes bool
}

// A Chaos is the interface with different methods to stop runnig containers.
type Chaos interface {
	StopContainers(container.Client, []string, string, interface{}) error
	KillContainers(container.Client, []string, string, interface{}) error
	RemoveContainers(container.Client, []string, string, interface{}) error
	NetemDelayContainers(container.Client, []string, string, interface{}) error
	PauseContainers(container.Client, []string, string, interface{}) error
	NetemLossRandomContainers(container.Client, []string, string, interface{}) error
	NetemLossStateContainers(container.Client, []string, string, interface{}) error
	NetemLossGEmodelContainers(container.Client, []string, string, interface{}) error
}

// NewChaos create new Pumba Choas instance
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
		// container name may start with forward slash, when using inspect fucntion
		if !matched {
			matched, err = regexp.MatchString(pattern, c.Name()[1:])
			if err != nil {
				return false
			}
		}
		return matched
	}
}

func listContainers(client container.Client, names []string, pattern string) ([]container.Container, error) {
	var err error
	var containers []container.Container
	if pattern != "" {
		if containers, err = client.ListContainers(regexContainerFilter(pattern)); err != nil {
			return nil, err
		}
	} else {
		if containers, err = client.ListContainers(containerFilter(names)); err != nil {
			return nil, err
		}
	}
	return containers, nil
}

func randomContainer(containers []container.Container) *container.Container {
	if containers != nil && len(containers) > 0 {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		i := r.Intn(len(containers))
		log.Debug(i, "  ", containers[i])
		return &containers[i]
	}
	return nil
}

func stopContainers(client container.Client, containers []container.Container, waitTime int) error {
	if waitTime == 0 {
		waitTime = DeafultWaitTime
	}
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			err := client.StopContainer(*container, waitTime, DryMode)
			if err != nil {
				return err
			}
		}
	} else {
		for _, container := range containers {
			err := client.StopContainer(container, waitTime, DryMode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func killContainers(client container.Client, containers []container.Container, signal string) error {
	if signal == "" {
		signal = DefaultKillSignal
	}
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			log.Debug("Container", container)
			err := client.KillContainer(*container, signal, DryMode)
			if err != nil {
				return err
			}
		}
	} else {
		for _, container := range containers {
			err := client.KillContainer(container, signal, DryMode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func removeContainers(client container.Client, containers []container.Container, force bool, links bool, volumes bool) error {
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			err := client.RemoveContainer(*container, force, links, volumes, DryMode)
			if err != nil {
				return err
			}
		}
	} else {
		for _, container := range containers {
			err := client.RemoveContainer(container, force, links, volumes, DryMode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func pauseContainers(client container.Client, containers []container.Container, duration time.Duration, stopChan <-chan bool) error {
	var err error
	pausedContainers := []container.Container{}
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			err = client.PauseContainer(*container, DryMode)
			if err != nil {
				return err
			}
			pausedContainers = append(pausedContainers, *container)
		}
	} else {
		for _, container := range containers {
			err = client.PauseContainer(container, DryMode)
			if err != nil {
				break
			} else {
				pausedContainers = append(pausedContainers, container)
			}
		}
	}
	// wait for specified duration and then unpause containers or unpause on signal to stopChan channel
	select {
	case <-stopChan:
		log.Debugf("Unpause containers by stop event")
		err = unpauseContainers(client, pausedContainers)
	case <-time.After(duration):
		log.Debugf("Unpause containers after: %s", duration)
		err = unpauseContainers(client, pausedContainers)
	}
	return err
}

func unpauseContainers(client container.Client, containers []container.Container) error {
	var err error
	for _, container := range containers {
		if e := client.UnpauseContainer(container, DryMode); e != nil {
			err = e
		}
	}
	return err // last non nil error
}

func netemContainers(client container.Client, containers []container.Container, netInterface string, netemCmd []string, ip net.IP, duration time.Duration, stopChan <-chan bool, tcimage string) error {
	var err error
	netemContainers := []container.Container{}
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			err = client.NetemContainer(*container, netInterface, netemCmd, ip, duration, tcimage, DryMode)
			if err != nil {
				return err
			}
			netemContainers = append(netemContainers, *container)
		}
	} else {
		for _, container := range containers {
			err = client.NetemContainer(container, netInterface, netemCmd, ip, duration, tcimage, DryMode)
			if err != nil {
				break
			} else {
				netemContainers = append(netemContainers, container)
			}
		}
	}
	// wait for specified duration and then stop netem (where it applied) or stop on signal to stopChan channel
	select {
	case <-stopChan:
		log.Debugf("Stopping netem by stop event")
		err = stopNetemContainers(client, netemContainers, netInterface, tcimage)
	case <-time.After(duration):
		log.Debugf("Stopping netem after: %s", duration)
		err = stopNetemContainers(client, netemContainers, netInterface, tcimage)
	}

	return err
}

func stopNetemContainers(client container.Client, containers []container.Container, netInterface string, tcimage string) error {
	var err error
	for _, container := range containers {
		if e := client.StopNetemContainer(container, netInterface, tcimage, DryMode); e != nil {
			err = e
		}
	}
	return err // last non nil error
}

//---------------------------------------------------------------------------------------------------

// StopContainers stop containers matching pattern
func (p pumbaChaos) StopContainers(client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("Stop containers")
	// get command details
	command, ok := cmd.(CommandStop)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandStop")
	}
	var err error
	var containers []container.Container
	if containers, err = listContainers(client, names, pattern); err != nil {
		return err
	}
	return stopContainers(client, containers, command.WaitTime)
}

// KillContainers - kill containers either by RE2 pattern (if specified) or by names
func (p pumbaChaos) KillContainers(client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("Kill containers")
	// get command details
	command, ok := cmd.(CommandKill)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandKill")
	}
	var err error
	var containers []container.Container
	if containers, err = listContainers(client, names, pattern); err != nil {
		return err
	}
	return killContainers(client, containers, command.Signal)
}

// RemoveContainers - remove container either by RE2 pattern (if specified) or by names
func (p pumbaChaos) RemoveContainers(client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("Remove containers")
	// get command details
	command, ok := cmd.(CommandRemove)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandRemove")
	}
	var err error
	var containers []container.Container
	if containers, err = listContainers(client, names, pattern); err != nil {
		return err
	}
	return removeContainers(client, containers, command.Force, command.Links, command.Volumes)
}

// NetemDelayContainers delay network traffic with optional Jitter and correlation
func (p pumbaChaos) NetemDelayContainers(client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("netem: dealy for containers")
	// get command details
	command, ok := cmd.(CommandNetemDelay)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandNetemDelay")
	}
	var err error
	var containers []container.Container
	if containers, err = listContainers(client, names, pattern); err != nil {
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

	return netemContainers(client, containers, command.NetInterface, netemCmd, command.IP, command.Duration, command.StopChan, command.Image)
}

func (p pumbaChaos) NetemLossRandomContainers(client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("netem: loss random for containers")
	// get command details
	command, ok := cmd.(CommandNetemLossRandom)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandNetemLossRandom")
	}
	var err error
	var containers []container.Container
	if containers, err = listContainers(client, names, pattern); err != nil {
		return err
	}
	// prepare netem loss command
	netemCmd := []string{"loss", strconv.FormatFloat(command.Percent, 'f', 2, 64)}
	if command.Correlation > 0 {
		netemCmd = append(netemCmd, strconv.FormatFloat(command.Correlation, 'f', 2, 64))
	}
	// run prepared netem loss command
	return netemContainers(client, containers, command.NetInterface, netemCmd, command.IP, command.Duration, command.StopChan, command.Image)
}

func (p pumbaChaos) NetemLossStateContainers(client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("netem: loss random for containers")
	// get command details
	command, ok := cmd.(CommandNetemLossState)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandNetemLossState")
	}
	var err error
	var containers []container.Container
	if containers, err = listContainers(client, names, pattern); err != nil {
		return err
	}
	// prepare netem loss state command
	netemCmd := []string{"loss", "state", strconv.FormatFloat(command.P13, 'f', 2, 64)}
	netemCmd = append(netemCmd, strconv.FormatFloat(command.P31, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(command.P32, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(command.P23, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(command.P14, 'f', 2, 64))
	// run prepared netem state command
	return netemContainers(client, containers, command.NetInterface, netemCmd, command.IP, command.Duration, command.StopChan, command.Image)
}

func (p pumbaChaos) NetemLossGEmodelContainers(client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("netem: loss random for containers")
	// get command details
	command, ok := cmd.(CommandNetemLossGEmodel)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandNetemLossGEmodel")
	}
	var err error
	var containers []container.Container
	if containers, err = listContainers(client, names, pattern); err != nil {
		return err
	}
	// prepare netem loss gemodel command
	netemCmd := []string{"loss", "gemodel", strconv.FormatFloat(command.PG, 'f', 2, 64)}
	netemCmd = append(netemCmd, strconv.FormatFloat(command.PB, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(command.OneH, 'f', 2, 64))
	netemCmd = append(netemCmd, strconv.FormatFloat(command.OneK, 'f', 2, 64))
	// run prepared netem loss gemodel command
	return netemContainers(client, containers, command.NetInterface, netemCmd, command.IP, command.Duration, command.StopChan, command.Image)
}

// PauseContainers pause container,if its name within `names`, for specified interval
func (p pumbaChaos) PauseContainers(client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Infof("Pause containers")
	// get command details
	command, ok := cmd.(CommandPause)
	if !ok {
		return errors.New("Unexpected cmd type; should be CommandPause")
	}
	var err error
	var containers []container.Container
	if containers, err = listContainers(client, names, pattern); err != nil {
		return err
	}
	return pauseContainers(client, containers, command.Duration, command.StopChan)
}
