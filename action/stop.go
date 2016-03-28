package actions

import (
	"regexp"

	log "github.com/Sirupsen/logrus"
	"github.com/gaia-adm/pumba/container"
)

const (
	deafultWaitTime   = 10
	defaultKillSignal = "SIGKILL"
)

// all containers beside Pumba
func allContainersFilter(c container.Container) bool {
	if c.IsPumba() {
		return false
	}
	return true
}

func containerFilter(names []string) container.Filter {
	if len(names) == 0 {
		return allContainersFilter
	}

	return func(c container.Container) bool {
		for _, name := range names {
			if (name == c.Name()) || (name == c.Name()[1:]) {
				return true
			}
		}
		return false
	}
}

func regexContainerFilter(pattern string) container.Filter {
	if pattern == "*" {
		return allContainersFilter
	}

	return func(c container.Container) bool {
		if c.IsPumba() {
			return false
		}
		matched, err := regexp.MatchString(pattern, c.Name())
		if err != nil {
			return false
		}
		return matched
	}
}

// StopByName stop container, if its name within `names`
func StopByName(client container.Client, names []string) error {
	log.Info("Stop containers by name")

	containers, err := client.ListContainers(containerFilter(names))
	if err != nil {
		return err
	}

	for _, container := range containers {
		err := client.StopContainer(container, deafultWaitTime)
		if err != nil {
			return err
		}
	}

	return nil
}

// StopByPattern stop containers matching pattern
func StopByPattern(client container.Client, pattern string) error {
	log.Info("Stop containers by pattern: ", pattern)

	containers, err := client.ListContainers(regexContainerFilter(pattern))
	if err != nil {
		return err
	}

	for _, container := range containers {
		err := client.StopContainer(container, deafultWaitTime)
		if err != nil {
			return err
		}
	}

	return nil
}

// KillByName kill container, if its name within `names`. You can pass custom
// signal, if empty SIGKILL will be used
func KillByName(client container.Client, names []string, signal string) error {
	log.Info("Kill containers by names")

	if signal == "" {
		signal = defaultKillSignal
	}

	containers, err := client.ListContainers(containerFilter(names))
	if err != nil {
		return err
	}

	for _, container := range containers {
		err := client.KillContainer(container, signal)
		if err != nil {
			return err
		}
	}

	return nil
}

// KillByPattern kill containers matching pattern. You can pass custom
// signal, if empty SIGKILL will be used
func KillByPattern(client container.Client, pattern string, signal string) error {
	log.Info("Kill containers matching pattern: ", pattern)

	if signal == "" {
		signal = defaultKillSignal
	}

	containers, err := client.ListContainers(regexContainerFilter(pattern))
	if err != nil {
		return err
	}

	for _, container := range containers {
		err := client.KillContainer(container, signal)
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoveByName remove container, if its name within `names`. Kill container if its running
// and `force` flag is `true`
func RemoveByName(client container.Client, names []string, force bool) error {
	log.Info("Remove containers by name")

	containers, err := client.ListContainers(containerFilter(names))
	if err != nil {
		return err
	}

	for _, container := range containers {
		err := client.RemoveContainer(container, force)
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoveByPattern remove container, if its name within `names`. Kill container if its running
// and `force` flag is `true`
func RemoveByPattern(client container.Client, pattern string, force bool) error {
	log.Info("Remove containers by pattern: ", pattern)

	containers, err := client.ListContainers(regexContainerFilter(pattern))
	if err != nil {
		return err
	}

	for _, container := range containers {
		err := client.RemoveContainer(container, force)
		if err != nil {
			return err
		}
	}

	return nil
}
