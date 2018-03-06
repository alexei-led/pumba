package container

import (
	"context"
	"net"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockClient mock struct
type MockClient struct {
	mock.Mock
}

// NewMockClient creates a new mock client
func NewMockClient() *MockClient {
	return &MockClient{}
}

// ListContainers mock
func (m *MockClient) ListContainers(ctx context.Context, cf Filter) ([]Container, error) {
	args := m.Called(ctx, cf)
	return args.Get(0).([]Container), args.Error(1)
}

// ListAllContainers mock
func (m *MockClient) ListAllContainers(ctx context.Context, cf Filter) ([]Container, error) {
	args := m.Called(ctx, cf)
	return args.Get(0).([]Container), args.Error(1)
}

// StopContainer mock
func (m *MockClient) StopContainer(ctx context.Context, c Container, timeout int, dryrun bool) error {
	args := m.Called(ctx, c, timeout)
	return args.Error(0)
}

// KillContainer mock
func (m *MockClient) KillContainer(ctx context.Context, c Container, s string, dryrun bool) error {
	args := m.Called(ctx, c, s)
	return args.Error(0)
}

// RemoveContainer mock
func (m *MockClient) RemoveContainer(ctx context.Context, c Container, f bool, l bool, v bool, dryrun bool) error {
	args := m.Called(ctx, c, f, l, v)
	return args.Error(0)
}

// PauseContainer mock
func (m *MockClient) PauseContainer(ctx context.Context, c Container, dryrun bool) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

// UnpauseContainer mock
func (m *MockClient) UnpauseContainer(ctx context.Context, c Container, dryrun bool) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

// NetemContainer mock
func (m *MockClient) NetemContainer(ctx context.Context, c Container, n string, s []string, ips []net.IP, d time.Duration, image string, dryrun bool) error {
	args := m.Called(ctx, c, n, s, ips, d, image)
	return args.Error(0)
}

// StopNetemContainer mock
func (m *MockClient) StopNetemContainer(ctx context.Context, c Container, n string, ips []net.IP, image string, dryrun bool) error {
	args := m.Called(ctx, c, n, ips, image)
	return args.Error(0)
}

// StartContainer mock
func (m *MockClient) StartContainer(ctx context.Context, c Container, dryrun bool) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}
