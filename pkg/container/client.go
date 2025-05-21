package container

import (
	"crypto/tls"
	"fmt"
)

// NewClient returns a new Client instance for the specified runtime.
// - runtimeType: "docker" or "containerd".
// - hostAddress: For "docker", this is the Docker host (e.g., "unix:///var/run/docker.sock").
//                For "containerd", this is the containerd address (e.g., "/run/containerd/containerd.sock").
// - namespace:   For "containerd", this is the containerd namespace (e.g., "k8s.io"). Ignored for "docker".
// - tlsConfig:   TLS configuration, primarily used for Docker client over HTTPS. Ignored for containerd unix socket.
func NewClient(runtimeType, hostAddress, namespace string, tlsConfig *tls.Config) (Client, error) {
	switch runtimeType {
	case "docker":
		// hostAddress for docker is dockerHost
		// namespace is ignored for docker client
		return NewDockerClient(hostAddress, tlsConfig)
	case "containerd":
		// hostAddress for containerd is containerdAddress
		// tlsConfig is typically nil or ignored for containerd if using unix socket.
		return NewContainerdClient(hostAddress, namespace)
	default:
		return nil, fmt.Errorf("unknown container runtime: %q - supported runtimes are 'docker' and 'containerd'", runtimeType)
	}
}
