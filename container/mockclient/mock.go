package mockclient

import (
	"time"

	"github.com/gaia-adm/pumba/container"
	"github.com/stretchr/testify/mock"
)

// MockClient mock struct
type MockClient struct {
	mock.Mock
}

// ListContainers mock
func (m *MockClient) ListContainers(cf container.Filter) ([]container.Container, error) {
	args := m.Called(cf)
	return args.Get(0).([]container.Container), args.Error(1)
}

// StopContainer mock
func (m *MockClient) StopContainer(c container.Container, timeout time.Duration) error {
	args := m.Called(c, timeout)
	return args.Error(0)
}

// StartContainer mock
func (m *MockClient) StartContainer(c container.Container) error {
	args := m.Called(c)
	return args.Error(0)
}

// RenameContainer mock
func (m *MockClient) RenameContainer(c container.Container, name string) error {
	args := m.Called(c, name)
	return args.Error(0)
}

// IsContainerStale mock
func (m *MockClient) IsContainerStale(c container.Container) (bool, error) {
	args := m.Called(c)
	return args.Bool(0), args.Error(1)
}

// RemoveImage mock
func (m *MockClient) RemoveImage(c container.Container, b bool) error {
	args := m.Called(c, b)
	return args.Error(0)
}
