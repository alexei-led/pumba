package docker

import (
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
)

func TestNewAPIClient(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{name: "unix socket", host: "unix:///var/run/docker.sock", wantErr: false},
		{name: "tcp host", host: "tcp://localhost:2375", wantErr: false},
		{name: "invalid scheme url", host: "://bad", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, err := NewAPIClient(tt.host, nil)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, api)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, api)
		})
	}
}

func TestNewFromAPI_NilReturnsError(t *testing.T) {
	c, err := NewFromAPI(nil)
	assert.Nil(t, c)
	assert.EqualError(t, err, "docker: api client must not be nil")
}

func TestNewFromAPI_ValidReturnsClient(t *testing.T) {
	api, err := NewAPIClient("unix:///var/run/docker.sock", nil)
	assert.NoError(t, err)
	assert.NotNil(t, api)

	c, err := NewFromAPI(api)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	var _ ctr.Client = c
}

func TestNewClient_DelegatesToHelpers(t *testing.T) {
	c, err := NewClient("unix:///var/run/docker.sock", nil)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	_, err = NewClient("://bad", nil)
	assert.Error(t, err)
}

func TestDockerClient_Close(t *testing.T) {
	c := dockerClient{}
	assert.NoError(t, c.Close())
}
