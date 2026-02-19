package netem

import (
	"context"
	"net"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
)

type mockNetemClient struct {
	mock.Mock
}

func (m *mockNetemClient) ListContainers(ctx context.Context, filter container.FilterFunc, opts container.ListOpts) ([]*container.Container, error) {
	args := m.Called(ctx, filter, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*container.Container), args.Error(1)
}

func (m *mockNetemClient) NetemContainer(ctx context.Context, c *container.Container, netInterface string, cmd []string, ips []*net.IPNet, sports, dports []string, duration time.Duration, tcimage string, pull, dryRun bool) error {
	args := m.Called(ctx, c, netInterface, cmd, ips, sports, dports, duration, tcimage, pull, dryRun)
	return args.Error(0)
}

func (m *mockNetemClient) StopNetemContainer(ctx context.Context, c *container.Container, netInterface string, ips []*net.IPNet, sports, dports []string, tcimage string, pull, dryRun bool) error {
	args := m.Called(ctx, c, netInterface, ips, sports, dports, tcimage, pull, dryRun)
	return args.Error(0)
}
