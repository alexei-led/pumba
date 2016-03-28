package actions

import (
	"testing"
	"time"

	"github.com/gaia-adm/pumba/container"
	"github.com/gaia-adm/pumba/container/mockclient"
	"github.com/samalba/dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPattern_Filter(t *testing.T) {
	c1 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:   "c1",
			Config: &dockerclient.ContainerConfig{},
		},
		nil,
	)
	c2 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:   "c2",
			Config: &dockerclient.ContainerConfig{},
		},
		nil,
	)
	cc := &dockerclient.ContainerConfig{
		Labels: map[string]string{"com.gaiaadm.pumba": "true"},
	}
	c3 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:   "c1",
			Config: cc,
		},
		nil,
	)
	cf := regexContainerFilter("*")
	assert.True(t, cf(c1))
	assert.True(t, cf(c2))
	assert.False(t, cf(c3))
}

func TestStopByPattern_All(t *testing.T) {
	c1 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name: "c1",
		},
		nil,
	)
	c2 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name: "c2",
		},
		nil,
	)
	cs := []container.Container{c1, c2}

	client := &mockclient.MockClient{}
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("StopContainer", c1, time.Duration(10)).Return(nil)
	client.On("StopContainer", c2, time.Duration(10)).Return(nil)

	err := StopByPattern(client, "*")

	assert.NoError(t, err)
	client.AssertExpectations(t)
}
