package docker

import (
	"bufio"
	"strings"

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

func NewMockEngine() *mocks.APIClient {
	return new(mocks.APIClient)
}

func mockAllContainers(_ *ctr.Container) bool {
	return true
}

func mockNoContainers(_ *ctr.Container) bool {
	return false
}
