package container

import (
	"context"
	"math/rand"
	"regexp"
	"time"
)

// ListOpts list options
type ListOpts struct {
	All    bool
	Labels []string
}

// Filter list filter
type Filter struct {
	Names   []string
	Pattern string
	Opts    ListOpts
}

func matchNames(names []string, containerName string) bool {
	for _, name := range names {
		// container name may start with forward slash, when using inspect function
		if (name == containerName) || (name == containerName[1:]) {
			return true
		}
	}
	return false
}

func matchPattern(pattern, containerName string) bool {
	matched, err := regexp.MatchString(pattern, containerName)
	if err != nil {
		return false
	}
	// container name may start with forward slash, when using inspect function
	if !matched {
		matched, err = regexp.MatchString(pattern, containerName[1:])
		if err != nil {
			return false
		}
	}
	return matched
}

func applyContainerFilter(filter Filter) FilterFunc {
	return func(c *Container) bool {
		// skip Pumba label
		if c.IsPumba() || c.IsPumbaSkip() {
			return false
		}
		// if not requested all
		if !filter.Opts.All {
			// match names
			if len(filter.Names) > 0 {
				return matchNames(filter.Names, c.containerInfo.Name)
			}
			return matchPattern(filter.Pattern, c.containerInfo.Name)
		}
		return true
	}
}

func listContainers(ctx context.Context, client Client, names []string, pattern string, labels []string, all bool) ([]*Container, error) {
	filter := Filter{
		Names:   names,
		Pattern: pattern,
		Opts: ListOpts{
			All:    all,
			Labels: labels,
		},
	}
	return client.ListContainers(ctx, applyContainerFilter(filter), filter.Opts)
}

// RandomContainer select random container
func RandomContainer(containers []*Container) *Container {
	if len(containers) > 0 {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		i := r.Intn(len(containers))
		return containers[i]
	}
	return nil
}

// ListNContainers list containers up to specified limit
func ListNContainers(ctx context.Context, client Client, names []string, pattern string, labels []string, limit int) ([]*Container, error) {
	containers, err := listContainers(ctx, client, names, pattern, labels, false)
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
