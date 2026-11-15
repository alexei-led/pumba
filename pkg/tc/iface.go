package tc

import "github.com/alexei-led/pumba/pkg/util"

// CheckInterface reuses the shared interface-name validation so the tc commands
// accept exactly the names the rest of pumba accepts.
func CheckInterface(name string) error {
	return util.ValidateInterfaceName(name)
}
