package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestID(t *testing.T) {
	c := Container{
		ContainerID: "foo",
		Labels:      map[string]string{},
		Networks:    map[string]NetworkLink{},
	}

	assert.Equal(t, "foo", c.ID())
}

func TestName(t *testing.T) {
	c := Container{
		ContainerName: "foo",
		Labels:        map[string]string{},
		Networks:      map[string]NetworkLink{},
	}

	assert.Equal(t, "foo", c.Name())
}

func TestImageName_Tagged(t *testing.T) {
	c := Container{
		Image:    "foo:latest",
		Labels:   map[string]string{},
		Networks: map[string]NetworkLink{},
	}

	assert.Equal(t, "foo:latest", c.ImageName())
}

func TestImageName_Untagged(t *testing.T) {
	c := Container{
		Image:    "foo",
		Labels:   map[string]string{},
		Networks: map[string]NetworkLink{},
	}

	assert.Equal(t, "foo:latest", c.ImageName())
}

func TestLinks(t *testing.T) {
	c := Container{
		Labels: map[string]string{},
		Networks: map[string]NetworkLink{
			"default": {Links: []string{"foo:foo", "bar:bar"}},
		},
	}

	links := c.Links()

	assert.Equal(t, []string{"foo", "bar"}, links)
}

func TestIsPumba_True(t *testing.T) {
	labels := map[string]string{
		"com.gaiaadm.pumba": "true",
	}

	c := Container{
		Labels:   labels,
		Networks: map[string]NetworkLink{},
	}

	assert.True(t, c.IsPumba())
}

func TestIsPumbaSkip_True(t *testing.T) {
	labels := map[string]string{
		"com.gaiaadm.pumba.skip": "true",
	}

	c := Container{
		Labels:   labels,
		Networks: map[string]NetworkLink{},
	}

	assert.True(t, c.IsPumbaSkip())
}

func TestIsPumba_WrongLabelValue(t *testing.T) {
	labels := map[string]string{
		"com.gaiaadm.pumba": "false",
	}

	c := Container{
		Labels:   labels,
		Networks: map[string]NetworkLink{},
	}

	assert.False(t, c.IsPumba())
}

func TestIsPumba_NoLabel(t *testing.T) {
	emptyLabels := map[string]string{}

	c := Container{
		Labels:   emptyLabels,
		Networks: map[string]NetworkLink{},
	}

	assert.False(t, c.IsPumba())
}

func TestStopSignal_Present(t *testing.T) {
	labels := map[string]string{
		"com.gaiaadm.pumba.stop-signal": "SIGQUIT",
	}

	c := Container{
		Labels:   labels,
		Networks: map[string]NetworkLink{},
	}

	assert.Equal(t, "SIGQUIT", c.StopSignal())
}

func TestStopSignal_NoLabel(t *testing.T) {
	emptyLabels := map[string]string{}

	c := Container{
		Labels:   emptyLabels,
		Networks: map[string]NetworkLink{},
	}

	assert.Equal(t, "", c.StopSignal())
}

func TestLinks_MultipleNetworks(t *testing.T) {
	c := Container{
		Labels: map[string]string{},
		Networks: map[string]NetworkLink{
			"frontend": {Links: []string{"api:api"}},
			"backend":  {Links: []string{"db:db", "cache:cache"}},
		},
	}
	links := c.Links()
	assert.Len(t, links, 3)
	assert.Contains(t, links, "api")
	assert.Contains(t, links, "db")
	assert.Contains(t, links, "cache")
}

func TestLinks_EmptyNetworks(t *testing.T) {
	c := Container{
		Labels:   map[string]string{},
		Networks: map[string]NetworkLink{},
	}
	assert.Nil(t, c.Links())
}

func TestLinks_NetworkWithNoLinks(t *testing.T) {
	c := Container{
		Labels: map[string]string{},
		Networks: map[string]NetworkLink{
			"bridge": {Links: nil},
		},
	}
	assert.Nil(t, c.Links())
}

func TestIsPumbaSkip_WrongValue(t *testing.T) {
	c := Container{
		Labels:   map[string]string{"com.gaiaadm.pumba.skip": "false"},
		Networks: map[string]NetworkLink{},
	}
	assert.False(t, c.IsPumbaSkip())
}

func TestIsPumbaSkip_NoLabel(t *testing.T) {
	c := Container{
		Labels:   map[string]string{},
		Networks: map[string]NetworkLink{},
	}
	assert.False(t, c.IsPumbaSkip())
}
