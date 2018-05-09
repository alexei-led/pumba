package action

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func makeContainersN(n int) ([]string, []container.Container) {
	names := make([]string, n)
	cs := make([]container.Container, n)
	for i := range cs {
		cs[i] = *container.NewContainer(
			container.ContainerDetailsResponse(container.AsMap("Name", "c"+strconv.Itoa(i))),
			container.ImageDetailsResponse(container.AsMap()),
		)
		names[i] = "c" + strconv.Itoa(i)
	}
	return names, cs
}

func TestPattern_DotRe2Filter(t *testing.T) {
	c1 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "c1")),
		container.ImageDetailsResponse(container.AsMap()),
	)
	c2 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "c2")),
		container.ImageDetailsResponse(container.AsMap()),
	)
	c3 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "c3")),
		container.ImageDetailsResponse(container.AsMap()),
	)

	cf := regexContainerFilter(".")
	assert.True(t, cf(c1))
	assert.True(t, cf(c2))
	assert.True(t, cf(c3))
}

func TestPattern_Re2Filter(t *testing.T) {
	c1 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "AbcEFG")),
		container.ImageDetailsResponse(container.AsMap()),
	)
	c2 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "AbcHKL")),
		container.ImageDetailsResponse(container.AsMap()),
	)
	c3 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap(
			"Name", "AbcPumba",
			"Labels", map[string]string{"com.gaiaadm.pumba": "true"},
		)),
		container.ImageDetailsResponse(container.AsMap()),
	)

	cf := regexContainerFilter("^Abc")
	assert.True(t, cf(c1))
	assert.True(t, cf(c2))
	assert.False(t, cf(c3))
}

func TestPattern_Re2FilterEnds(t *testing.T) {
	c1 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "AbcEFG-result-1")),
		container.ImageDetailsResponse(container.AsMap()),
	)
	c2 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "AbcHKL-ignore")),
		container.ImageDetailsResponse(container.AsMap()),
	)
	c3 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "AbcPumba-result-2")),
		container.ImageDetailsResponse(container.AsMap()),
	)

	cf := regexContainerFilter("(.+)result")
	assert.True(t, cf(c1))
	assert.False(t, cf(c2))
	assert.True(t, cf(c3))
}

func TestNamesFilter(t *testing.T) {
	c1 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "ccc")),
		container.ImageDetailsResponse(container.AsMap()),
	)
	c2 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "ddd")),
		container.ImageDetailsResponse(container.AsMap()),
	)
	c3 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap(
			"Name", "xxx",
			"Labels", map[string]string{"com.gaiaadm.pumba": "true"},
		)),
		container.ImageDetailsResponse(container.AsMap()),
	)
	cf := container.ContainerFilter([]string{"ccc", "bbb", "xxx"})
	assert.True(t, cf(c1))
	assert.False(t, cf(c2))
	assert.False(t, cf(c3))
}

func TestAllNamesFilter(t *testing.T) {
	c1 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "ccc")),
		container.ImageDetailsResponse(container.AsMap()),
	)
	c2 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "ddd")),
		container.ImageDetailsResponse(container.AsMap()),
	)
	c3 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap(
			"Name", "xxx",
			"Labels", map[string]string{"com.gaiaadm.pumba": "true"},
		)),
		container.ImageDetailsResponse(container.AsMap()),
	)
	cf := container.ContainerFilter([]string{})
	assert.True(t, cf(c1))
	assert.True(t, cf(c2))
	assert.False(t, cf(c3))
}

func TestAllFilter(t *testing.T) {
	c1 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "ccc")),
		container.ImageDetailsResponse(container.AsMap()),
	)
	c2 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap("Name", "ddd")),
		container.ImageDetailsResponse(container.AsMap()),
	)
	c3 := *container.NewContainer(
		container.ContainerDetailsResponse(container.AsMap(
			"Name", "xxx",
			"Labels", map[string]string{"com.gaiaadm.pumba": "true"},
		)),
		container.ImageDetailsResponse(container.AsMap()),
	)

	assert.True(t, container.AllContainersFilter(c1))
	assert.True(t, container.AllContainersFilter(c2))
	assert.False(t, container.AllContainersFilter(c3))
}

func TestNetemRateByName(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandNetemRate{
		NetInterface:   "eth1",
		IPs:            nil,
		Duration:       1 * time.Millisecond,
		Rate:           "300kbit",
		PacketOverhead: 10,
		CellSize:       20,
		CellOverhead:   30,
	}
	client := new(container.MockClient)
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"rate", "300kbit", "10", "20", "30"}, []net.IP(nil), 1*time.Millisecond, "", false).Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", []net.IP(nil), "", false).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemRateContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestSelectRandomContainer(t *testing.T) {
	_, cs := makeContainersN(30)
	c1 := randomContainer(cs)
	time.Sleep(1 * time.Millisecond)
	c2 := randomContainer(cs)
	assert.NotNil(t, c1)
	assert.NotNil(t, c2)
	assert.NotEqual(t, c1.Name(), c2.Name())
}
