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
	StopByName(container.Client, []string) error
	StopByPattern(container.Client, string) error
	KillByName(container.Client, []string, string) error
	KillByPattern(container.Client, string, string) error
	RemoveByName(container.Client, []string, bool) error
	RemoveByPattern(container.Client, string, bool) error
	DisruptByName(container.Client, []string, string) error
	DisruptByPattern(container.Client, string, string) error
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

func randomContainer(containers []container.Container) *container.Container {
	if containers != nil && len(containers) > 0 {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		i := r.Intn(len(containers))
		log.Debug(i, "  ", containers[i])
		return &containers[i]
	}
	return nil
}

func stopContainers(client container.Client, containers []container.Container) error {
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			err := client.StopContainer(*container, deafultWaitTime, DryMode)
			if err != nil {
				return err
			}
		}
	} else {
		for _, container := range containers {
			err := client.StopContainer(container, deafultWaitTime, DryMode)
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

func removeContainers(client container.Client, containers []container.Container, force bool) error {
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			err := client.RemoveContainer(*container, force, DryMode)
			if err != nil {
				return err
			}
		}
	} else {
		for _, container := range containers {
			err := client.RemoveContainer(container, force, DryMode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func disruptContainers(client container.Client, containers []container.Container) error {
	if RandomMode {
		container := randomContainer(containers)
		if container != nil {
			err := client.DisruptContainer(*container, DryMode)
			if err != nil {
				return err
			}
		}
	} else {
		for _, container := range containers {
			err := client.DisruptContainer(container, DryMode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
//---------------------------------------------------------------------------------------------------

// StopByName stop container, if its name within `names`
func (p Pumba) StopByName(client container.Client, names []string) error {
	log.Info("Stop containers by names: ", names)
	containers, err := client.ListContainers(containerFilter(names))
	if err != nil {
		return err
	}
	return stopContainers(client, containers)
}

// StopByPattern stop containers matching pattern
func (p Pumba) StopByPattern(client container.Client, pattern string) error {
	log.Infof("Stop containers by RE2 regex: '%s'", pattern)
	containers, err := client.ListContainers(regexContainerFilter(pattern))
	if err != nil {
		return err
	}
	return stopContainers(client, containers)
}

// KillByName kill container, if its name within `names`. You can pass custom
// signal, if empty SIGKILL will be used
func (p Pumba) KillByName(client container.Client, names []string, signal string) error {
	log.Info("Kill containers by names: ", names)

	containers, err := client.ListContainers(containerFilter(names))
	if err != nil {
		return err
	}
	return killContainers(client, containers, signal)
}

// KillByPattern kill containers matching pattern. You can pass custom
// signal, if empty SIGKILL will be used
func (p Pumba) KillByPattern(client container.Client, pattern string, signal string) error {
	log.Infof("Kill containers matching RE2 regex: '%s'", pattern)
	containers, err := client.ListContainers(regexContainerFilter(pattern))
	if err != nil {
		return err
	}
	return killContainers(client, containers, signal)
}

// RemoveByName remove container, if its name within `names`. Kill container if its running
// and `force` flag is `true`
func (p Pumba) RemoveByName(client container.Client, names []string, force bool) error {
	log.Info("Remove containers by names: ", names)
	containers, err := client.ListContainers(containerFilter(names))
	if err != nil {
		return err
	}
	return removeContainers(client, containers, force)
}

// RemoveByPattern remove container, if its name within `names`. Kill container if its running
// and `force` flag is `true`
func (p Pumba) RemoveByPattern(client container.Client, pattern string, force bool) error {
	log.Infof("Remove containers by RE2 pattern: '%s'", pattern)
	containers, err := client.ListContainers(regexContainerFilter(pattern))
	if err != nil {
		return err
	}
	return removeContainers(client, containers, force)
}

// DisruptByName disrupts container egress network, if its name within `names`. 
// Disruption is currently limited to delayed response
func (p Pumba) DisruptByName(client container.Client, names []string, netemCmd string) error {
	log.Info("Disrupt containers by names: ", names)
	containers, err := client.ListContainers(containerFilter(names))
	if err != nil {
		return err
	}
	return disruptContainers(client, containers, netemCmd)
}

// DisruptByPattern disrupts container egress network, if its name matches 'pattern'. 
// Disruption is currently limited to delayed response
func (p Pumba) DisruptByPattern(client container.Client, pattern string, netemCmd string) error {
	log.Infof("Disrupt containers by RE2 pattern: '%s'", pattern)
	containers, err := client.ListContainers(regexContainerFilter(pattern))
	if err != nil {
		return err
	}
	return disruptContainers(client, containers, netemCmd)
}
