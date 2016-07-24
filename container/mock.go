package container

import (
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

// StartContainer mock
func (m *MockClient) StartContainer(c Container) error {
	args := m.Called(c)
	return args.Error(0)
}

// RenameContainer mock
func (m *MockClient) RenameContainer(c Container, name string) error {
	args := m.Called(c, name)
	return args.Error(0)
}

// RemoveImage mock
func (m *MockClient) RemoveImage(c Container, b bool, dryrun bool) error {
	args := m.Called(c, b)
	return args.Error(0)
}

// KillContainer mock
func (m *MockClient) KillContainer(c Container, s string, dryrun bool) error {
	args := m.Called(c, s)
	return args.Error(0)
}

// RemoveContainer mock
func (m *MockClient) RemoveContainer(c Container, f bool, l string, v string, dryrun bool) error {
	args := m.Called(c, f, l, v)
	return args.Error(0)
}

// PauseContainer mock
func (m *MockClient) PauseContainer(c Container, d time.Duration, dryrun bool) error {
	args := m.Called(c, d)
	return args.Error(0)
}

// DisruptContainer mock
func (m *MockClient) DisruptContainer(c Container, n string, s string, dryrun bool) error {
	args := m.Called(c, n, s)
	return args.Error(0)
}
