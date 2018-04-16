package docker

import "context"

// ChaosCommand Docker chaos command
type ChaosCommand interface {
	Run(ctx context.Context, random bool) error
}
