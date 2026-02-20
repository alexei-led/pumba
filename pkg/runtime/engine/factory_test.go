package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient_ContainerdNotImplemented(t *testing.T) {
	client, err := NewClient("containerd", "", nil)
	assert.Nil(t, client)
	assert.EqualError(t, err, `runtime "containerd" is not implemented`)
}

func TestNewClient_DockerDispatch(t *testing.T) {
	client, err := NewClient("docker", "unix:///nonexistent.sock", nil)
	if err != nil {
		// Docker daemon unavailable is fine — verify we reached docker.NewClient
		assert.NotContains(t, err.Error(), "unknown runtime")
		assert.NotContains(t, err.Error(), "not implemented")
	} else {
		assert.NotNil(t, client)
	}
}

func TestNewClient_CaseInsensitive(t *testing.T) {
	client, err := NewClient("DOCKER", "unix:///nonexistent.sock", nil)
	if err != nil {
		assert.NotContains(t, err.Error(), "unknown runtime")
	} else {
		assert.NotNil(t, client)
	}
}

func TestNewClient_UnknownRuntime(t *testing.T) {
	client, err := NewClient("cri-o", "", nil)
	assert.Nil(t, client)
	assert.EqualError(t, err, `unknown runtime "cri-o": supported runtimes are "docker" and "containerd"`)
}
