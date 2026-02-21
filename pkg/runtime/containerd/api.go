package containerd

import (
	"context"

	containerd "github.com/containerd/containerd/v2/client"
)

// apiClient defines the subset of *containerd.Client methods used by containerdClient.
// This enables unit testing with mock implementations.
type apiClient interface {
	Containers(ctx context.Context, filters ...string) ([]containerd.Container, error)
	LoadContainer(ctx context.Context, id string) (containerd.Container, error)
}
