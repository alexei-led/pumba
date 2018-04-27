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

func TestNetemDelayByName(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandNetemDelay{
		NetInterface: "eth1",
		IPs:          nil,
		Duration:     1 * time.Millisecond,
		Time:         120,
		Jitter:       25,
		Correlation:  0.23,
	}
	client := new(container.MockClient)
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"delay", "120ms", "25ms", "0.23"}, []net.IP(nil), 1*time.Millisecond, "", false).Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", []net.IP(nil), "", false).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemDelayContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemDelayByNameRandom(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandNetemDelay{
		NetInterface: "eth1",
		IPs:          nil,
		Duration:     1 * time.Millisecond,
		Time:         120,
		Jitter:       25,
		Correlation:  5.5,
		Distribution: "uniform",
	}
	client := new(container.MockClient)
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	client.On("NetemContainer", mock.Anything, mock.Anything, "eth1", []string{"delay", "120ms", "25ms", "5.50", "distribution", "uniform"}, []net.IP(nil), 1*time.Millisecond, "", false).Return(nil)
	client.On("StopNetemContainer", mock.Anything, mock.Anything, "eth1", []net.IP(nil), "", false).Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.NetemDelayContainers(context.TODO(), client, names, "", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemDelayByPattern(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandNetemDelay{
		NetInterface: "eth1",
		IPs:          nil,
		Duration:     1 * time.Millisecond,
		Time:         120,
		Jitter:       25,
		Correlation:  15,
	}
	client := new(container.MockClient)
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"delay", "120ms", "25ms", "15.00"}, []net.IP(nil), 1*time.Millisecond, "", false).Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", []net.IP(nil), "", false).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemDelayContainers(context.TODO(), client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemDelayByPatternIPFilterOne(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	ips := []net.IP{net.ParseIP("10.10.0.1")}
	cmd := CommandNetemDelay{
		NetInterface: "eth1",
		IPs:          ips,
		Duration:     1 * time.Millisecond,
		Time:         120,
		Jitter:       25,
		Correlation:  10,
	}
	client := new(container.MockClient)
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"delay", "120ms", "25ms", "10.00"}, ips, 1*time.Millisecond, "", false).Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", ips, "", false).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemDelayContainers(context.TODO(), client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemDelayByPatternIPFilterMany(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	ips := []net.IP{net.ParseIP("10.10.0.1"), net.ParseIP("10.10.0.3")}
	cmd := CommandNetemDelay{
		NetInterface: "eth1",
		IPs:          ips,
		Duration:     1 * time.Millisecond,
		Time:         120,
		Jitter:       25,
		Correlation:  10,
	}
	client := new(container.MockClient)
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"delay", "120ms", "25ms", "10.00"}, ips, 1*time.Millisecond, "", false).Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", ips, "", false).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemDelayContainers(context.TODO(), client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemDelayByPatternRandom(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandNetemDelay{
		NetInterface: "eth1",
		IPs:          nil,
		Duration:     1 * time.Millisecond,
		Time:         120,
		Jitter:       25,
		Correlation:  10.2,
	}
	client := new(container.MockClient)
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	client.On("NetemContainer", mock.Anything, mock.Anything, "eth1", []string{"delay", "120ms", "25ms", "10.20"}, []net.IP(nil), 1*time.Millisecond, "", false).Return(nil)
	client.On("StopNetemContainer", mock.Anything, mock.Anything, "eth1", []net.IP(nil), "", false).Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.NetemDelayContainers(context.TODO(), client, []string{}, "^c", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemLossByName(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandNetemLossRandom{
		NetInterface: "eth1",
		IPs:          nil,
		Duration:     1 * time.Millisecond,
		Percent:      11.5,
		Correlation:  25.53,
	}
	client := new(container.MockClient)
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"loss", "11.50", "25.53"}, []net.IP(nil), 1*time.Millisecond, "", false).Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", []net.IP(nil), "", false).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemLossRandomContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemLossStateByName(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandNetemLossState{
		NetInterface: "eth1",
		IPs:          nil,
		Duration:     1 * time.Millisecond,
		P13:          11.5,
		P31:          12.6,
		P32:          13.7,
		P23:          14.8,
		P14:          15.9,
	}
	client := new(container.MockClient)
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"loss", "state", "11.50", "12.60", "13.70", "14.80", "15.90"}, []net.IP(nil), 1*time.Millisecond, "", false).Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", []net.IP(nil), "", false).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemLossStateContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemLossGEmodelByName(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandNetemLossGEmodel{
		NetInterface: "eth1",
		IPs:          nil,
		Duration:     1 * time.Millisecond,
		PG:           11.5,
		PB:           12.6,
		OneH:         13.7,
		OneK:         14.8,
	}
	client := new(container.MockClient)
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"loss", "gemodel", "11.50", "12.60", "13.70", "14.80"}, []net.IP(nil), 1*time.Millisecond, "", false).Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", []net.IP(nil), "", false).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemLossGEmodelContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
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
