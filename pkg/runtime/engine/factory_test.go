package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient_ContainerdNotImplemented(t *testing.T) {
	client, err := NewClient("containerd", "", nil)
	assert.Nil(t, client)
	assert.EqualError(t, err, `runtime "containerd" is not yet implemented`)
}

func TestNewClient_DockerDispatch(t *testing.T) {
	// Docker SDK creates clients lazily (no dial at construction time),
	// so NewClient succeeds even with a nonexistent socket.
	client, err := NewClient("docker", "unix:///nonexistent.sock", nil)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_CaseInsensitive(t *testing.T) {
	client, err := NewClient("DOCKER", "unix:///nonexistent.sock", nil)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_UnknownRuntime(t *testing.T) {
	client, err := NewClient("cri-o", "", nil)
	assert.Nil(t, client)
	assert.EqualError(t, err, `unknown runtime "cri-o": supported runtimes are "docker"`)
}
