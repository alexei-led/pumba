package action

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/gaia-adm/pumba/container"
	"github.com/samalba/dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func makeContainersN(n int) ([]string, []container.Container) {
	names := make([]string, n)
	cs := make([]container.Container, n)
	for i := range cs {
		cs[i] = *container.NewContainer(
			&dockerclient.ContainerInfo{
				Name: "c" + strconv.Itoa(i),
			},
			nil,
		)
		names[i] = "c" + strconv.Itoa(i)
	}
	return names, cs
}

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

func TestAllNamesFilter(t *testing.T) {
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
	cf := containerFilter([]string{})
	assert.True(t, cf(c1))
	assert.True(t, cf(c2))
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
	// prepare test data and mock
	names, cs := makeContainersN(10)
	cmd := CommandStop{WaitTime: 10}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	for _, c := range cs {
		client.On("StopContainer", c, 10).Return(nil)
	}
	// doc action
	pumba := pumbaChaos{}
	err := pumba.StopContainers(client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestStopByNameRandom(t *testing.T) {
	// prepare test data and mock
	names, cs := makeContainersN(10)
	cmd := CommandStop{WaitTime: 10}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("StopContainer", mock.AnythingOfType("container.Container"), 10).Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.StopContainers(client, names, "", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestStopByPattern(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandStop{WaitTime: 10}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	for _, c := range cs {
		client.On("StopContainer", c, 10).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.StopContainers(client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestStopByPatternRandom(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandStop{WaitTime: 10}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("StopContainer", mock.AnythingOfType("container.Container"), 10).Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.StopContainers(client, []string{}, "^c", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestKillByName(t *testing.T) {
	// prepare test data and mock
	names, cs := makeContainersN(10)
	cmd := CommandKill{Signal: "SIGTEST"}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	for _, c := range cs {
		client.On("KillContainer", c, "SIGTEST").Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.KillContainers(client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestKillByNameRandom(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandKill{Signal: "SIGTEST"}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("KillContainer", mock.AnythingOfType("container.Container"), "SIGTEST").Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.KillContainers(client, names, "", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestKillByPattern(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandKill{Signal: "SIGTEST"}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	for i := range cs {
		client.On("KillContainer", cs[i], "SIGTEST").Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.KillContainers(client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestKillByPatternRandom(t *testing.T) {
	// prepare test data and mock
	_, cs := makeContainersN(10)
	cmd := CommandKill{Signal: "SIGTEST"}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("KillContainer", mock.AnythingOfType("container.Container"), "SIGTEST").Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.KillContainers(client, []string{}, "^c", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByName(t *testing.T) {
	names, cs := makeContainersN(10)
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	cmd := CommandRemove{Force: false, Links: false, Volumes: false}
	for _, c := range cs {
		client.On("RemoveContainer", c, false, false, false).Return(nil)
	}
	pumba := pumbaChaos{}
	err := pumba.RemoveContainers(client, names, "", cmd)
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByNameRandom(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	client := container.NewMockSamalbaClient()
	cmd := CommandRemove{Force: false, Links: true, Volumes: true}
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("RemoveContainer", mock.AnythingOfType("container.Container"), false, true, true).Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.RemoveContainers(client, names, "", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByPattern(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandRemove{Force: false, Links: true, Volumes: true}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	for _, c := range cs {
		client.On("RemoveContainer", c, false, true, true).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.RemoveContainers(client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByPatternRandom(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandRemove{Force: false, Links: true, Volumes: true}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("RemoveContainer", mock.AnythingOfType("container.Container"), false, true, true).Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.RemoveContainers(client, []string{}, "^c", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestPauseByName(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandPause{Duration: 2 * time.Millisecond}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	for _, c := range cs {
		client.On("PauseContainer", c, 2*time.Millisecond).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.PauseContainers(client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestPauseByPattern(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandPause{Duration: 2 * time.Millisecond}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	for _, c := range cs {
		client.On("PauseContainer", c, 2*time.Millisecond).Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.PauseContainers(client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestPauseByNameRandom(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandPause{Duration: 2 * time.Millisecond}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("PauseContainer", mock.AnythingOfType("container.Container"), 2*time.Millisecond).Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.PauseContainers(client, names, "", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemDealyByName(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandNetemDelay{
		NetInterface: "eth1",
		IP:           nil,
		Duration:     1 * time.Second,
		Amount:       120,
		Variation:    25,
		Correlation:  15,
	}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", c, "eth1", "delay 120ms 25ms 15%", net.ParseIP(""), 1*time.Second).Return(nil)
		client.On("StopNetemContainer", c, "eth1").Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemDelayContainers(client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemDealyByNameRandom(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	cmd := CommandNetemDelay{
		NetInterface: "eth1",
		IP:           nil,
		Duration:     1 * time.Second,
		Amount:       120,
		Variation:    25,
		Correlation:  15,
	}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("NetemContainer", mock.AnythingOfType("container.Container"), "eth1", "delay 120ms 25ms 15%", net.ParseIP(""), 1*time.Second).Return(nil)
	client.On("StopNetemContainer", mock.AnythingOfType("container.Container"), "eth1").Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.NetemDelayContainers(client, names, "", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemDealyByPattern(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandNetemDelay{
		NetInterface: "eth1",
		IP:           nil,
		Duration:     1 * time.Second,
		Amount:       120,
		Variation:    25,
		Correlation:  15,
	}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", c, "eth1", "delay 120ms 25ms 15%", net.ParseIP(""), 1*time.Second).Return(nil)
		client.On("StopNetemContainer", c, "eth1").Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemDelayContainers(client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemDealyByPatternIPFilter(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	ip := net.ParseIP("10.10.0.1")
	cmd := CommandNetemDelay{
		NetInterface: "eth1",
		IP:           ip,
		Duration:     1 * time.Second,
		Amount:       120,
		Variation:    25,
		Correlation:  15,
	}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	for _, c := range cs {
		client.On("NetemContainer", c, "eth1", "delay 120ms 25ms 15%", ip, 1*time.Second).Return(nil)
		client.On("StopNetemContainer", c, "eth1").Return(nil)
	}
	// do action
	pumba := pumbaChaos{}
	err := pumba.NetemDelayContainers(client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemDealyByPatternRandom(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandNetemDelay{
		NetInterface: "eth1",
		IP:           nil,
		Duration:     1 * time.Second,
		Amount:       120,
		Variation:    25,
		Correlation:  15,
	}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("NetemContainer", mock.AnythingOfType("container.Container"), "eth1", "delay 120ms 25ms 15%", net.ParseIP(""), 1*time.Second).Return(nil)
	client.On("StopNetemContainer", mock.AnythingOfType("container.Container"), "eth1").Return(nil)
	// do action
	RandomMode = true
	pumba := pumbaChaos{}
	err := pumba.NetemDelayContainers(client, []string{}, "^c", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestSelectRandomContainer(t *testing.T) {
	_, cs := makeContainersN(30)
	c1 := randomContainer(cs)
	c2 := randomContainer(cs)
	assert.NotNil(t, c1)
	assert.NotNil(t, c2)
	assert.NotEqual(t, c1.Name(), c2.Name())
}
