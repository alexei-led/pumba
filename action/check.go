package actions

import (
	"sort"

	"github.com/gaia-adm/pumba/container"
)

func pumbaContainersFilter(c container.Container) bool { return c.IsPumba() }

// CheckPrereqs will ensure that there are not multiple instances of the
// Pumba running simultaneously. If multiple Pumba containers are
// detected, this function will stop and remove all but the most recently
// started container.
func CheckPrereqs(client container.Client, cleanup bool) error {
	containers, err := client.ListContainers(pumbaContainersFilter)
	if err != nil {
		return err
	}

	if len(containers) > 1 {
		sort.Sort(container.ByCreated(containers))

		// Iterate over all containers execept the last one
		for _, c := range containers[0 : len(containers)-1] {
			if err := client.StopContainer(c, 60, false); err != nil {
				return err
			}

			if cleanup {
				if err := client.RemoveImage(c, true, false); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
