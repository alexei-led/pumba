package engine

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/docker"
)

// NewClient creates a container.Client for the specified runtime.
// Supported runtimes: "docker". The "containerd" runtime is planned but not yet implemented.
func NewClient(runtimeName, host string, tlsConfig *tls.Config) (container.Client, error) {
	switch strings.ToLower(runtimeName) {
	case "docker":
		return docker.NewClient(host, tlsConfig)
	case "containerd":
		return nil, fmt.Errorf("runtime %q is not yet implemented", runtimeName)
	default:
		return nil, fmt.Errorf("unknown runtime %q: supported runtimes are \"docker\"", runtimeName)
	}
}
