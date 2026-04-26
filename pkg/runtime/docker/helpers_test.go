package docker

import (
	"bufio"
	"strings"
	"testing"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/docker/docker/api/types"
)

// fakeExecAttach returns a HijackedResponse suitable for mocking
// ContainerExecAttach in tests that don't care about the exec stream output.
func fakeExecAttach() types.HijackedResponse {
	conn := &mockConn{}
	conn.On("Close").Return(nil)
	return types.HijackedResponse{
		Conn:   conn,
		Reader: bufio.NewReader(strings.NewReader("")),
	}
}

// NewMockEngine returns a mock APIClient bound to t so AssertExpectations runs
// at cleanup. Pass t to catch mismatched EXPECT() calls automatically.
func NewMockEngine(t *testing.T) *mocks.APIClient {
	t.Helper()
	return mocks.NewAPIClient(t)
}

func mockAllContainers(_ *ctr.Container) bool {
	return true
}

func mockNoContainers(_ *ctr.Container) bool {
	return false
}
