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

func TestPattern_DotRe2Filter(t *testing.T) {
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
	c3 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:   "c3",
			Config: &dockerclient.ContainerConfig{},
		},
		nil,
	)
	cf := regexContainerFilter(".")
	assert.True(t, cf(c1))
	assert.True(t, cf(c2))
	assert.True(t, cf(c3))
}

func TestPattern_Re2Filter(t *testing.T) {
	c1 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:   "AbcEFG",
			Config: &dockerclient.ContainerConfig{},
		},
		nil,
	)
	c2 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:   "AbcHKL",
			Config: &dockerclient.ContainerConfig{},
		},
		nil,
	)
	cc := &dockerclient.ContainerConfig{
		Labels: map[string]string{"com.gaiaadm.pumba": "true"},
	}
	c3 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:   "AbcPumba",
			Config: cc,
		},
		nil,
	)
	cf := regexContainerFilter("^Abc")
	assert.True(t, cf(c1))
	assert.True(t, cf(c2))
	assert.False(t, cf(c3))
}

func TestNamesFilter(t *testing.T) {
	c1 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:   "ccc",
			Config: &dockerclient.ContainerConfig{},
		},
		nil,
	)
	c2 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:   "ddd",
			Config: &dockerclient.ContainerConfig{},
		},
		nil,
	)
	cc := &dockerclient.ContainerConfig{
		Labels: map[string]string{"com.gaiaadm.pumba": "true"},
	}
	c3 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:   "xxx",
			Config: cc,
		},
		nil,
	)
	cf := containerFilter([]string{"ccc", "bbb", "xxx"})
	assert.True(t, cf(c1))
	assert.False(t, cf(c2))
	assert.False(t, cf(c3))
}

func TestAllFilter(t *testing.T) {
	c1 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:   "ccc",
			Config: &dockerclient.ContainerConfig{},
		},
		nil,
	)
	c2 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:   "ddd",
			Config: &dockerclient.ContainerConfig{},
		},
		nil,
	)
	cc := &dockerclient.ContainerConfig{
		Labels: map[string]string{"com.gaiaadm.pumba": "true"},
	}
	c3 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:   "xxx",
			Config: cc,
		},
		nil,
	)
	assert.True(t, allContainersFilter(c1))
	assert.True(t, allContainersFilter(c2))
	assert.False(t, allContainersFilter(c3))
}

func TestStopByName(t *testing.T) {
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

	err := Pumba{}.StopByName(client, []string{})

	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestStopByPattern(t *testing.T) {
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

	err := Pumba{}.StopByPattern(client, "^c")

	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestKillByName(t *testing.T) {
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
	client.On("KillContainer", c1, "SIGTEST").Return(nil)
	client.On("KillContainer", c2, "SIGTEST").Return(nil)

	err := Pumba{}.KillByName(client, []string{"c1", "c2"}, "SIGTEST")

	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestKillByPattern(t *testing.T) {
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
	client.On("KillContainer", c1, "SIGTEST").Return(nil)
	client.On("KillContainer", c2, "SIGTEST").Return(nil)

	err := Pumba{}.KillByPattern(client, "^c", "SIGTEST")

	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByName(t *testing.T) {
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
	client.On("RemoveContainer", c1, false).Return(nil)
	client.On("RemoveContainer", c2, false).Return(nil)

	err := Pumba{}.RemoveByName(client, []string{"c1", "c2"}, false)

	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByPattern(t *testing.T) {
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
	client.On("RemoveContainer", c1, false).Return(nil)
	client.On("RemoveContainer", c2, false).Return(nil)

	err := Pumba{}.RemoveByPattern(client, "^c", false)

	assert.NoError(t, err)
	client.AssertExpectations(t)
}
