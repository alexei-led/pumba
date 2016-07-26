package action

import (
	"errors"
	"math/rand"
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
}

// CommandNetemDelay arguments for 'netem delay' sub-command
type CommandNetemDelay struct {
	NetInterface string
	Duration     time.Duration
	Amount       int
	Variation    int
	Correlation  int
}

// CommandStop arguments for stop command
type CommandStop struct {
	WaitTime int
}

// CommandRemove arguments for remove command
type CommandRemove struct {
	Force   bool
	Link    string
	Volumes string
}

// A Chaos is the interface with different methods to stop runnig containers.
type Chaos interface {
	StopContainers(container.Client, []string, string, interface{}) error
	KillContainers(container.Client, []string, string, interface{}) error
	RemoveContainers(container.Client, []string, string, interface{}) error
	NetemDelayContainers(container.Client, []string, string, interface{}) error
	PauseContainers(container.Client, []string, string, interface{}) error
}

// Pumba makes Chaos
type Pumba struct{}

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

func removeContainers(client container.Client, containers []container.Container, force bool, link string, volumes string) error {
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			err := client.RemoveContainer(*container, force, link, volumes, DryMode)
			if err != nil {
				return err
			}
		}
	} else {
		for _, container := range containers {
			err := client.RemoveContainer(container, force, link, volumes, DryMode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func pauseContainers(client container.Client, containers []container.Container, duration time.Duration) error {
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			err := client.PauseContainer(*container, duration, DryMode)
			if err != nil {
				return err
			}
		}
	} else {
		for _, container := range containers {
			err := client.PauseContainer(container, duration, DryMode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func disruptContainers(client container.Client, containers []container.Container, netInterface string, netemCmd string) error {
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			err := client.DisruptContainer(*container, netInterface, netemCmd, DryMode)
			if err != nil {
				return err
			}
		}
	} else {
		for _, container := range containers {
			err := client.DisruptContainer(container, netInterface, netemCmd, DryMode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

//---------------------------------------------------------------------------------------------------

// StopContainers stop containers matching pattern
func (p Pumba) StopContainers(client container.Client, names []string, pattern string, cmd interface{}) error {
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
func (p Pumba) KillContainers(client container.Client, names []string, pattern string, cmd interface{}) error {
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
func (p Pumba) RemoveContainers(client container.Client, names []string, pattern string, cmd interface{}) error {
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
	return removeContainers(client, containers, command.Force, command.Link, command.Volumes)
}

// NetemDelayContainers delay network traffic with optional variation and correlation
func (p Pumba) NetemDelayContainers(client container.Client, names []string, pattern string, cmd interface{}) error {
	log.Info("netem dealy for containers")
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
	netemCmd := "delay " + strconv.Itoa(command.Amount) + "ms"
	if command.Variation > 0 {
		netemCmd += " " + strconv.Itoa(command.Variation) + "ms"
	}
	if command.Correlation > 0 {
		netemCmd += " " + strconv.Itoa(command.Correlation) + "%"
	}

	return disruptContainers(client, containers, command.NetInterface, netemCmd)
}

// PauseContainers pause container,if its name within `names`, for specified interval
func (p Pumba) PauseContainers(client container.Client, names []string, pattern string, cmd interface{}) error {
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
	return pauseContainers(client, containers, command.Duration)
}
