package action

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/gaia-adm/pumba/container"
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
	cf := containerFilter([]string{"ccc", "bbb", "xxx"})
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
	cf := containerFilter([]string{})
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

	assert.True(t, allContainersFilter(c1))
	assert.True(t, allContainersFilter(c2))
	assert.False(t, allContainersFilter(c3))
}

func TestStopByName(t *testing.T) {
	// prepare test data and mock
	names, cs := makeContainersN(10)
	cmd := CommandStop{WaitTime: 10}
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("StopContainer", mock.Anything, c, 10).Return(nil)
	}
	// doc action
	pumba := pumbaChaos{}
	err := pumba.StopContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestStopByNameRandom(t *testing.T) {
	// prepare test data and mock
	names, cs := makeContainersN(10)
	cmd := CommandStop{WaitTime: 10}
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	client.On("StopContainer", mock.Anything, mock.Anything, 10).Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.StopContainers(context.TODO(), client, names, "", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestStopByPattern(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandStop{WaitTime: 10}
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("StopContainer", mock.Anything, c, 10).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.StopContainers(context.TODO(), client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestStopByPatternRandom(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandStop{WaitTime: 10}
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	client.On("StopContainer", mock.Anything, mock.Anything, 10).Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.StopContainers(context.TODO(), client, []string{}, "^c", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestKillByName(t *testing.T) {
	// prepare test data and mock
	names, cs := makeContainersN(10)
	cmd := CommandKill{Signal: "SIGTEST"}
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("KillContainer", mock.Anything, c, "SIGTEST").Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.KillContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestKillByNameRandom(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandKill{Signal: "SIGTEST"}
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	client.On("KillContainer", mock.Anything, mock.Anything, "SIGTEST").Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.KillContainers(context.TODO(), client, names, "", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestKillByPattern(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandKill{Signal: "SIGTEST"}
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for i := range cs {
		client.On("KillContainer", mock.Anything, cs[i], "SIGTEST").Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.KillContainers(context.TODO(), client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestKillByPatternRandom(t *testing.T) {
	// prepare test data and mock
	_, cs := makeContainersN(10)
	cmd := CommandKill{Signal: "SIGTEST"}
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	client.On("KillContainer", mock.Anything, mock.Anything, "SIGTEST").Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.KillContainers(context.TODO(), client, []string{}, "^c", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByName(t *testing.T) {
	names, cs := makeContainersN(10)
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	cmd := CommandRemove{Force: false, Links: false, Volumes: false}
	for _, c := range cs {
		client.On("RemoveContainer", mock.Anything, c, false, false, false).Return(nil)
	}
	pumba := pumbaChaos{}
	err := pumba.RemoveContainers(context.TODO(), client, names, "", cmd)
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByNameRandom(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	client := container.NewMockClient()
	cmd := CommandRemove{Force: false, Links: true, Volumes: true}
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	client.On("RemoveContainer", mock.Anything, mock.Anything, false, true, true).Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.RemoveContainers(context.TODO(), client, names, "", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByPattern(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandRemove{Force: false, Links: true, Volumes: true}
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("RemoveContainer", mock.Anything, c, false, true, true).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.RemoveContainers(context.TODO(), client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByPatternRandom(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandRemove{Force: false, Links: true, Volumes: true}
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	client.On("RemoveContainer", mock.Anything, mock.Anything, false, true, true).Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.RemoveContainers(context.TODO(), client, []string{}, "^c", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestPauseByName(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandPause{Duration: 2 * time.Millisecond}
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("PauseContainer", mock.Anything, c).Return(nil)
		client.On("UnpauseContainer", mock.Anything, c).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.PauseContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestPauseByPattern(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandPause{Duration: 2 * time.Millisecond}
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("PauseContainer", mock.Anything, c).Return(nil)
		client.On("UnpauseContainer", mock.Anything, c).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.PauseContainers(context.TODO(), client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestPauseByNameRandom(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandPause{Duration: 2 * time.Millisecond}
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	client.On("PauseContainer", mock.Anything, mock.Anything).Return(nil)
	client.On("UnpauseContainer", mock.Anything, mock.Anything).Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.PauseContainers(context.TODO(), client, names, "", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
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
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"delay", "120ms", "25ms", "0.23"}, []net.IP(nil), 1*time.Millisecond, "").Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", []net.IP(nil), "").Return(nil)
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
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	client.On("NetemContainer", mock.Anything, mock.Anything, "eth1", []string{"delay", "120ms", "25ms", "5.50", "distribution", "uniform"}, []net.IP(nil), 1*time.Millisecond, "").Return(nil)
	client.On("StopNetemContainer", mock.Anything, mock.Anything, "eth1", []net.IP(nil), "").Return(nil)
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
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"delay", "120ms", "25ms", "15.00"}, []net.IP(nil), 1*time.Millisecond, "").Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", []net.IP(nil), "").Return(nil)
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
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"delay", "120ms", "25ms", "10.00"}, ips, 1*time.Millisecond, "").Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", ips, "").Return(nil)
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
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"delay", "120ms", "25ms", "10.00"}, ips, 1*time.Millisecond, "").Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", ips, "").Return(nil)
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
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	client.On("NetemContainer", mock.Anything, mock.Anything, "eth1", []string{"delay", "120ms", "25ms", "10.20"}, []net.IP(nil), 1*time.Millisecond, "").Return(nil)
	client.On("StopNetemContainer", mock.Anything, mock.Anything, "eth1", []net.IP(nil), "").Return(nil)
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
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"loss", "11.50", "25.53"}, []net.IP(nil), 1*time.Millisecond, "").Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", []net.IP(nil), "").Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemLossRandomContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemLossBadCommand(t *testing.T) {
	// prepare test data and mocks
	names, _ := makeContainersN(10)
	cmd := CommandKill{
		Signal: "xuyak",
	}
	client := container.NewMockClient()
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemLossRandomContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.Error(t, err)
	assert.EqualError(t, err, "Unexpected cmd type; should be CommandNetemLossRandom")
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
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"loss", "state", "11.50", "12.60", "13.70", "14.80", "15.90"}, []net.IP(nil), 1*time.Millisecond, "").Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", []net.IP(nil), "").Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemLossStateContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemLossStateBadCommand(t *testing.T) {
	// prepare test data and mocks
	names, _ := makeContainersN(10)
	cmd := CommandKill{
		Signal: "xuyak",
	}
	client := container.NewMockClient()
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemLossStateContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.Error(t, err)
	assert.EqualError(t, err, "Unexpected cmd type; should be CommandNetemLossState")
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
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"loss", "gemodel", "11.50", "12.60", "13.70", "14.80"}, []net.IP(nil), 1*time.Millisecond, "").Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", []net.IP(nil), "").Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemLossGEmodelContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemLossGEmodelBadCommand(t *testing.T) {
	// prepare test data and mocks
	names, _ := makeContainersN(10)
	cmd := CommandKill{
		Signal: "xuyak",
	}
	client := container.NewMockClient()
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemLossGEmodelContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.Error(t, err)
	assert.EqualError(t, err, "Unexpected cmd type; should be CommandNetemLossGEmodel")
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
	client := container.NewMockClient()
	client.On("ListContainers", mock.Anything, mock.Anything).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", mock.Anything, c, "eth1", []string{"rate", "300kbit", "10", "20", "30"}, []net.IP(nil), 1*time.Millisecond, "").Return(nil)
		client.On("StopNetemContainer", mock.Anything, c, "eth1", []net.IP(nil), "").Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemRateContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemRateBadCommand(t *testing.T) {
	// prepare test data and mocks
	names, _ := makeContainersN(10)
	cmd := CommandKill{
		Signal: "xuyak",
	}
	client := container.NewMockClient()
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemRateContainers(context.TODO(), client, names, "", cmd)
	// asserts
	assert.Error(t, err)
	assert.EqualError(t, err, "Unexpected cmd type; should be CommandNetemRate")
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
