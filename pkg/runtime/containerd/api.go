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
	GetImage(ctx context.Context, ref string) (containerd.Image, error)
	Pull(ctx context.Context, ref string, opts ...containerd.RemoteOpt) (containerd.Image, error)
	NewContainer(ctx context.Context, id string, opts ...containerd.NewContainerOpts) (containerd.Container, error)
	Close() error
}
