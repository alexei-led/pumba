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

func TestNewClient_UnknownRuntime(t *testing.T) {
	client, err := NewClient("cri-o", "", nil)
	assert.Nil(t, client)
	assert.EqualError(t, err, `unknown runtime "cri-o": supported runtimes are "docker" and "containerd"`)
}
