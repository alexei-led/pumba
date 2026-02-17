package container

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
)

// ListOpts list options
type ListOpts struct {
	All    bool
	Labels []string
}

// list filter
type filter struct {
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

func applyContainerFilter(flt filter) FilterFunc {
	return func(c *Container) bool {
		// skip Pumba label
		if c.IsPumba() || c.IsPumbaSkip() {
			return false
		}
		// match names
		if len(flt.Names) > 0 {
			return matchNames(flt.Names, c.ContainerInfo.Name)
		}
		return matchPattern(flt.Pattern, c.ContainerInfo.Name)
	}
}

func listContainers(ctx context.Context, client Client, names []string, pattern string, labels []string, all bool) ([]*Container, error) {
	f := filter{
		Names:   names,
		Pattern: pattern,
		Opts: ListOpts{
			All:    all,
			Labels: labels,
		},
	}
	containers, err := client.ListContainers(ctx, applyContainerFilter(f), f.Opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	return containers, nil
}

// RandomContainer select random container
func RandomContainer(containers []*Container) *Container {
	if len(containers) > 0 {
		return containers[rand.Intn(len(containers))] //nolint:gosec
	}
	return nil
}

// ListNContainers list containers up to specified limit
func ListNContainers(ctx context.Context, client Client, names []string, pattern string, labels []string, limit int) ([]*Container, error) {
	return ListNContainersAll(ctx, client, names, pattern, labels, limit, false)
}

// ListNContainersAll list containers up to specified limit, optionally including stopped containers
func ListNContainersAll(ctx context.Context, client Client, names []string, pattern string, labels []string, limit int, all bool) ([]*Container, error) {
	containers, err := listContainers(ctx, client, names, pattern, labels, all)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(containers) > limit {
		rand.Shuffle(len(containers), func(i, j int) {
			containers[i], containers[j] = containers[j], containers[i]
		})
		containers = containers[0:limit]
	}

	return containers, nil
}
