package docker

import (
	"context"
	"math/rand"
	"regexp"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
)

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

func listContainers(ctx context.Context, client container.Client, names []string, pattern string, all bool) ([]container.Container, error) {
	var filter container.Filter

	if pattern != "" {
		filter = regexContainerFilter(pattern)
	} else {
		filter = containerFilter(names)
	}

	if all {
		return client.ListAllContainers(ctx, filter)
	} else {
		return client.ListContainers(ctx, filter)
	}
}

func randomContainer(containers []container.Container) *container.Container {
	if len(containers) > 0 {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		i := r.Intn(len(containers))
		return &containers[i]
	}
	return nil
}

func listRunningContainers(ctx context.Context, client container.Client, names []string, pattern string) ([]container.Container, error) {
	return listContainers(ctx, client, names, pattern, false)
}

func listNContainers(ctx context.Context, client container.Client, names []string, pattern string, n int) ([]container.Container, error) {
	containers, err := listRunningContainers(ctx, client, names, pattern)
	if err != nil {
		return nil, err
	}

	if len(containers) > n && n > 0 {
		for i := range containers {
			j := rand.Intn(i + 1)
			containers[i], containers[j] = containers[j], containers[i]
		}
		return containers[0:n], nil
	}

	return containers, nil
}
