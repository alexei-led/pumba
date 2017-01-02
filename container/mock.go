package container

import (
	"net"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockClient mock struct
type MockClient struct {
	mock.Mock
}

// NewMockSamalbaClient creates a new mock client
func NewMockSamalbaClient() *MockClient {
	return &MockClient{}
}

// ListContainers mock
func (m *MockClient) ListContainers(cf Filter) ([]Container, error) {
	args := m.Called(cf)
	return args.Get(0).([]Container), args.Error(1)
}

// StopContainer mock
func (m *MockClient) StopContainer(c Container, timeout int, dryrun bool) error {
	args := m.Called(c, timeout)
	return args.Error(0)
}

// KillContainer mock
func (m *MockClient) KillContainer(c Container, s string, dryrun bool) error {
	args := m.Called(c, s)
	return args.Error(0)
}

// RemoveContainer mock
func (m *MockClient) RemoveContainer(c Container, f bool, l bool, v bool, dryrun bool) error {
	args := m.Called(c, f, l, v)
	return args.Error(0)
}

// PauseContainer mock
func (m *MockClient) PauseContainer(c Container, dryrun bool) error {
	args := m.Called(c)
	return args.Error(0)
}

// UnpauseContainer mock
func (m *MockClient) UnpauseContainer(c Container, dryrun bool) error {
	args := m.Called(c)
	return args.Error(0)
}

// NetemContainer mock
func (m *MockClient) NetemContainer(c Container, n string, s []string, ip net.IP, d time.Duration, image string, dryrun bool) error {
	args := m.Called(c, n, s, ip, d, image)
	return args.Error(0)
}

// StopNetemContainer mock
func (m *MockClient) StopNetemContainer(c Container, n string, image string, dryrun bool) error {
	args := m.Called(c, n, image)
	return args.Error(0)
}
