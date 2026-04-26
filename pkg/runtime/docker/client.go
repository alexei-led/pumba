package docker

import (
	"crypto/tls"
	"errors"
	"fmt"

	ctr "github.com/alexei-led/pumba/pkg/container"
	dockerapi "github.com/docker/docker/client"
)

// NewClient returns a new Client instance which can be used to interact with the Docker API.
func NewClient(dockerHost string, tlsConfig *tls.Config) (ctr.Client, error) {
	apiClient, err := NewAPIClient(dockerHost, tlsConfig)
	if err != nil {
		return nil, err
	}
	return NewFromAPI(apiClient)
}

// NewAPIClient returns a bare Docker SDK client. Exposed so alternate runtimes
// (e.g. Podman via the Docker-compat socket) can reuse the HTTP/TLS setup.
func NewAPIClient(dockerHost string, tlsConfig *tls.Config) (*dockerapi.Client, error) {
	httpClient, err := HTTPClient(dockerHost, tlsConfig)
	if err != nil {
		return nil, err
	}

	apiClient, err := dockerapi.NewClientWithOpts(dockerapi.WithHost(dockerHost), dockerapi.WithHTTPClient(httpClient), dockerapi.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return apiClient, nil
}

// NewFromAPI wraps an existing Docker SDK client as a ctr.Client. Exposed so
// alternate runtimes can reuse the Docker implementation via embedding.
func NewFromAPI(api *dockerapi.Client) (ctr.Client, error) {
	if api == nil {
		return nil, errors.New("docker: api client must not be nil")
	}
	return dockerClient{containerAPI: api, imageAPI: api, systemAPI: api}, nil
}

type dockerClient struct {
	containerAPI dockerapi.ContainerAPIClient
	imageAPI     dockerapi.ImageAPIClient
	systemAPI    dockerapi.SystemAPIClient
}

// Close is a no-op for the Docker client; the underlying HTTP connections are managed by the SDK.
func (client dockerClient) Close() error { return nil }
