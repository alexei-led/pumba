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
func (m *MockClient) StopContainer(c container.Container, timeout int, dryrun bool) error {
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

// RemoveImage mock
func (m *MockClient) RemoveImage(c container.Container, b bool, dryrun bool) error {
	args := m.Called(c, b)
	return args.Error(0)
}

// KillContainer mock
func (m *MockClient) KillContainer(c container.Container, s string, dryrun bool) error {
	args := m.Called(c, s)
	return args.Error(0)
}

// RemoveContainer mock
func (m *MockClient) RemoveContainer(c container.Container, f bool, l string, v string, dryrun bool) error {
	args := m.Called(c, f, l, v)
	return args.Error(0)
}

// PauseContainer mock
func (m *MockClient) PauseContainer(c container.Container, d time.Duration, dryrun bool) error {
	args := m.Called(c, d)
	return args.Error(0)
}

// DisruptContainer mock
func (m *MockClient) DisruptContainer(c container.Container, s string, dryrun bool) error {
	args := m.Called(c, s)
	return args.Error(0)
}
