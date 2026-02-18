package container

import "regexp"

// matchNames checks if containerName matches any of the provided names.
// Container names may start with a forward slash when using inspect function.
func matchNames(names []string, containerName string) bool {
	for _, name := range names {
		if name == containerName {
			return true
		}
		// container name may start with forward slash
		if containerName != "" && name == containerName[1:] {
			return true
		}
	}
	return false
}

// matchPattern checks if containerName matches the given regex pattern.
// Container names may start with a forward slash when using inspect function.
func matchPattern(pattern, containerName string) bool {
	matched, err := regexp.MatchString(pattern, containerName)
	if err != nil {
		return false
	}
	if !matched && containerName != "" {
		matched, err = regexp.MatchString(pattern, containerName[1:])
		if err != nil {
			return false
		}
	}
	return matched
}

// applyContainerFilter creates a FilterFunc from a filter config.
func applyContainerFilter(flt filter) FilterFunc {
	return func(c *Container) bool {
		// skip Pumba label
		if c.IsPumba() || c.IsPumbaSkip() {
			return false
		}
		// match names
		if len(flt.Names) > 0 {
			return matchNames(flt.Names, c.ContainerName)
		}
		return matchPattern(flt.Pattern, c.ContainerName)
	}
}
