package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/network"
	"github.com/docker/engine-api/types/container"
)

func TestID(t *testing.T) {
	c := Container{
		containerInfo: types.ContainerJSON{ContainerJSONBase: &types.ContainerJSONBase{ID: "foo"}},
	}

	assert.Equal(t, "foo", c.ID())
}

func TestName(t *testing.T) {
	c := Container{
		containerInfo: types.ContainerJSON{ContainerJSONBase: &types.ContainerJSONBase{Name: "foo"}},
	}

	assert.Equal(t, "foo", c.Name())
}

func TestImageID(t *testing.T) {
	c := Container{
		imageInfo: types.ImageInspect{ID: "foo"},
	}

	assert.Equal(t, "foo", c.ImageID())
}

func TestImageName_Tagged(t *testing.T) {
	c := Container{
		containerInfo: types.ContainerJSON{ContainerJSONBase: &types.ContainerJSONBase{Image: "foo:latest"}},
	}

	assert.Equal(t, "foo:latest", c.ImageName())
}

func TestImageName_Untagged(t *testing.T) {
	c := Container{
		containerInfo: types.ContainerJSON{ContainerJSONBase: &types.ContainerJSONBase{Image: "foo:latest"}},
	}

	assert.Equal(t, "foo:latest", c.ImageName())
}

func TestLinks(t *testing.T) {
	networks := map[string]*network.EndpointSettings{
		"default": {Links: []string{"foo:foo", "bar:bar"}},
	}

	c := Container{
		containerInfo: types.ContainerJSON{NetworkSettings: &types.NetworkSettings{Networks: networks}},
	}

	links := c.Links()

	assert.Equal(t, []string{"foo", "bar"}, links)
}

func TestIsPumba_True(t *testing.T) {
	labels := map[string]string{
		"com.gaiaadm.pumba": "true",
	}

	c := Container{
		containerInfo: types.ContainerJSON{Config: &container.Config{Labels: labels}},
	}

	assert.True(t, c.IsPumba())
}

func TestIsPumbaSkip_True(t *testing.T) {
	labels := map[string]string{
		"com.gaiaadm.pumba.skip": "true",
	}

	c := Container{
		containerInfo: types.ContainerJSON{Config: &container.Config{Labels: labels}},
	}

	assert.True(t, c.IsPumbaSkip())
}

func TestIsPumba_WrongLabelValue(t *testing.T) {
	labels := map[string]string{
		"com.gaiaadm.pumba": "false",
	}

	c := Container{
		containerInfo: types.ContainerJSON{Config: &container.Config{Labels: labels}},
	}

	assert.False(t, c.IsPumba())
}

func TestIsPumba_NoLabel(t *testing.T) {
	emptyLabels := map[string]string{}

	c := Container{
		containerInfo: types.ContainerJSON{Config: &container.Config{Labels: emptyLabels}},
	}

	assert.False(t, c.IsPumba())
}

func TestStopSignal_Present(t *testing.T) {
	labels := map[string]string{
		"com.gaiaadm.pumba.stop-signal": "SIGQUIT",
	}

	c := Container{
		containerInfo: types.ContainerJSON{Config: &container.Config{Labels: labels}},
	}

	assert.Equal(t, "SIGQUIT", c.StopSignal())
}

func TestStopSignal_NoLabel(t *testing.T) {
	emptyLabels := map[string]string{}

	c := Container{
		containerInfo: types.ContainerJSON{Config: &container.Config{Labels: emptyLabels}},
	}

	assert.Equal(t, "", c.StopSignal())
}
