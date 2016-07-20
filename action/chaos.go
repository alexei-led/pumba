package actions

import (
	"math/rand"
	"regexp"
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
	deafultWaitTime   = 10
	defaultKillSignal = "SIGKILL"
)

// A Chaos is the interface with different methods to stop runnig containers.
type Chaos interface {
	StopContainers(container.Client, []string, string, int) error
	KillContainers(container.Client, []string, string, string) error
	RemoveContainers(container.Client, []string, string, bool, string, string) error
	NetemContainers(container.Client, []string, string, string) error
	PauseContainers(container.Client, []string, string, time.Duration) error
}

// Pumba makes chaos
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
		waitTime = deafultWaitTime
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
		signal = defaultKillSignal
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

func disruptContainers(client container.Client, containers []container.Container, netemCmd string) error {
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			err := client.DisruptContainer(*container, netemCmd, DryMode)
			if err != nil {
				return err
			}
		}
	} else {
		for _, container := range containers {
			err := client.DisruptContainer(container, netemCmd, DryMode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

//---------------------------------------------------------------------------------------------------

// StopContainers stop containers matching pattern
func (p Pumba) StopContainers(client container.Client, names []string, pattern string, waitTime int) error {
	log.Info("Stop containers")
	var err error
	var containers []container.Container
	if containers, err = listContainers(client, names, pattern); err != nil {
		return err
	}
	return stopContainers(client, containers, waitTime)
}

// KillContainers - kill containers either by RE2 pattern (if specified) or by names
func (p Pumba) KillContainers(client container.Client, names []string, pattern string, signal string) error {
	log.Info("Kill containers")
	var err error
	var containers []container.Container
	if containers, err = listContainers(client, names, pattern); err != nil {
		return err
	}
	return killContainers(client, containers, signal)
}

// RemoveContainers - remove container either by RE2 pattern (if specified) or by names
func (p Pumba) RemoveContainers(client container.Client, names []string, pattern string, force bool, link string, volumes string) error {
	log.Info("Remove containers")
	var err error
	var containers []container.Container
	if containers, err = listContainers(client, names, pattern); err != nil {
		return err
	}
	return removeContainers(client, containers, force, link, volumes)
}

// NetemContainers disrupts container egress network, if its name within `names`.
// Disruption is currently limited to delayed response
func (p Pumba) NetemContainers(client container.Client, names []string, pattern string, netemCmd string) error {
	log.Info("Disrupt containers")
	var err error
	var containers []container.Container
	if containers, err = listContainers(client, names, pattern); err != nil {
		return err
	}
	return disruptContainers(client, containers, netemCmd)
}

// PauseContainers pause container,if its name within `names`, for specified interval
func (p Pumba) PauseContainers(client container.Client, names []string, pattern string, duration time.Duration) error {
	log.Infof("Pause containers")
	var err error
	var containers []container.Container
	if containers, err = listContainers(client, names, pattern); err != nil {
		return err
	}
	return pauseContainers(client, containers, duration)
}
