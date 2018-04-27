package container

import (
	"context"
	"math/rand"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
)

// GetIntervalValue get interval value from string duration
func GetIntervalValue(interval string) (time.Duration, error) {
	// get recurrent time interval
	if interval == "" {
		log.Debug("no interval specified, running only once")
		return 0, nil
	} else if i, err := time.ParseDuration(interval); err == nil {
		return i, nil
	} else {
		return 0, err
	}
}

// AllContainersFilter all containers beside Pumba and PumbaSkip
func AllContainersFilter(c Container) bool {
	if c.IsPumba() || c.IsPumbaSkip() {
		return false
	}
	return true
}

func ContainerFilter(names []string) Filter {
	if len(names) == 0 {
		return AllContainersFilter
	}

	return func(c Container) bool {
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

func RegexContainerFilter(pattern string) Filter {
	return func(c Container) bool {
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

func ListContainers(ctx context.Context, client Client, names []string, pattern string, all bool) ([]Container, error) {
	var filter Filter

	if pattern != "" {
		filter = RegexContainerFilter(pattern)
	} else {
		filter = ContainerFilter(names)
	}

	if all {
		return client.ListAllContainers(ctx, filter)
	}
	return client.ListContainers(ctx, filter)
}

func RandomContainer(containers []Container) *Container {
	if len(containers) > 0 {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		i := r.Intn(len(containers))
		return &containers[i]
	}
	return nil
}

func ListRunningContainers(ctx context.Context, client Client, names []string, pattern string) ([]Container, error) {
	return ListContainers(ctx, client, names, pattern, false)
}

func ListNContainers(ctx context.Context, client Client, names []string, pattern string, limit int) ([]Container, error) {
	containers, err := ListRunningContainers(ctx, client, names, pattern)
	if err != nil {
		return nil, err
	}

	if len(containers) > limit && limit > 0 {
		for i := range containers {
			j := rand.Intn(i + 1)
			containers[i], containers[j] = containers[j], containers[i]
		}
		return containers[0:limit], nil
	}

	return containers, nil
}
