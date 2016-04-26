package container

import (
	"testing"

	"github.com/samalba/dockerclient"
	"github.com/stretchr/testify/assert"
)

func TestID(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{Id: "foo"},
	}

	assert.Equal(t, "foo", c.ID())
}

func TestName(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{Name: "foo"},
	}

	assert.Equal(t, "foo", c.Name())
}

func TestImageID(t *testing.T) {
	c := Container{
		imageInfo: &dockerclient.ImageInfo{
			Id: "foo",
		},
	}

	assert.Equal(t, "foo", c.ImageID())
}

func TestImageName_Tagged(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Config: &dockerclient.ContainerConfig{
				Image: "foo:latest",
			},
		},
	}

	assert.Equal(t, "foo:latest", c.ImageName())
}

func TestImageName_Untagged(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Config: &dockerclient.ContainerConfig{
				Image: "foo",
			},
		},
	}

	assert.Equal(t, "foo:latest", c.ImageName())
}

func TestLinks(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			HostConfig: &dockerclient.HostConfig{
				Links: []string{"foo:foo", "bar:bar"},
			},
		},
	}

	links := c.Links()

	assert.Equal(t, []string{"foo", "bar"}, links)
}

func TestIsPumba_True(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Config: &dockerclient.ContainerConfig{
				Labels: map[string]string{"com.gaiaadm.pumba": "true"},
			},
		},
	}

	assert.True(t, c.IsPumba())
}

func TestIsPumbaSkip_True(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Config: &dockerclient.ContainerConfig{
				Labels: map[string]string{"com.gaiaadm.pumba.skip": "true"},
			},
		},
	}

	assert.True(t, c.IsPumbaSkip())
}

func TestIsPumba_WrongLabelValue(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Config: &dockerclient.ContainerConfig{
				Labels: map[string]string{"com.gaiaadm.pumba": "false"},
			},
		},
	}

	assert.False(t, c.IsPumba())
}

func TestIsPumba_NoLabel(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Config: &dockerclient.ContainerConfig{
				Labels: map[string]string{},
			},
		},
	}

	assert.False(t, c.IsPumba())
}

func TestStopSignal_Present(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Config: &dockerclient.ContainerConfig{
				Labels: map[string]string{
					"com.gaiaadm.pumba.stop-signal": "SIGQUIT",
				},
			},
		},
	}

	assert.Equal(t, "SIGQUIT", c.StopSignal())
}

func TestStopSignal_NoLabel(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Config: &dockerclient.ContainerConfig{
				Labels: map[string]string{},
			},
		},
	}

	assert.Equal(t, "", c.StopSignal())
}
