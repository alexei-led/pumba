package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestID(t *testing.T) {
	c := Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "foo")),
	}

	assert.Equal(t, "foo", c.ID())
}

func TestName(t *testing.T) {
	c := Container{
		ContainerInfo: DetailsResponse(AsMap("Name", "foo")),
	}

	assert.Equal(t, "foo", c.Name())
}

func TestImageID(t *testing.T) {
	c := Container{
		ImageInfo: ImageDetailsResponse(AsMap("ID", "foo")),
	}

	assert.Equal(t, "foo", c.ImageID())
}

func TestImageName_Tagged(t *testing.T) {
	c := Container{
		ContainerInfo: DetailsResponse(AsMap("Image", "foo:latest")),
	}

	assert.Equal(t, "foo:latest", c.ImageName())
}

func TestImageName_Untagged(t *testing.T) {
	c := Container{
		ContainerInfo: DetailsResponse(AsMap("Image", "foo")),
	}

	assert.Equal(t, "foo:latest", c.ImageName())
}

func TestLinks(t *testing.T) {
	c := Container{
		ContainerInfo: DetailsResponse(AsMap("Links", []string{"foo:foo", "bar:bar"})),
	}

	links := c.Links()

	assert.Equal(t, []string{"foo", "bar"}, links)
}

func TestIsPumba_True(t *testing.T) {
	labels := map[string]string{
		"com.gaiaadm.pumba": "true",
	}

	c := Container{
		ContainerInfo: DetailsResponse(AsMap("Labels", labels)),
	}

	assert.True(t, c.IsPumba())
}

func TestIsPumbaSkip_True(t *testing.T) {
	labels := map[string]string{
		"com.gaiaadm.pumba.skip": "true",
	}

	c := Container{
		ContainerInfo: DetailsResponse(AsMap("Labels", labels)),
	}

	assert.True(t, c.IsPumbaSkip())
}

func TestIsPumba_WrongLabelValue(t *testing.T) {
	labels := map[string]string{
		"com.gaiaadm.pumba": "false",
	}

	c := Container{
		ContainerInfo: DetailsResponse(AsMap("Labels", labels)),
	}

	assert.False(t, c.IsPumba())
}

func TestIsPumba_NoLabel(t *testing.T) {
	emptyLabels := map[string]string{}

	c := Container{
		ContainerInfo: DetailsResponse(AsMap("Labels", emptyLabels)),
	}

	assert.False(t, c.IsPumba())
}

func TestStopSignal_Present(t *testing.T) {
	labels := map[string]string{
		"com.gaiaadm.pumba.stop-signal": "SIGQUIT",
	}

	c := Container{
		ContainerInfo: DetailsResponse(AsMap("Labels", labels)),
	}

	assert.Equal(t, "SIGQUIT", c.StopSignal())
}

func TestStopSignal_NoLabel(t *testing.T) {
	emptyLabels := map[string]string{}

	c := Container{
		ContainerInfo: DetailsResponse(AsMap("Labels", emptyLabels)),
	}

	assert.Equal(t, "", c.StopSignal())
}
