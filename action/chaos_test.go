package action

import (
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
	err := Pumba{}.StopContainers(client, names, "", cmd)
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
	err := Pumba{}.StopContainers(client, names, "", cmd)
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
	err := Pumba{}.StopContainers(client, []string{}, "^c", cmd)
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
	err := Pumba{}.StopContainers(client, []string{}, "^c", cmd)
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
	err := Pumba{}.KillContainers(client, names, "", cmd)
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
	err := Pumba{}.KillContainers(client, names, "", cmd)
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
	err := Pumba{}.KillContainers(client, []string{}, "^c", cmd)
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
	err := Pumba{}.KillContainers(client, []string{}, "^c", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByName(t *testing.T) {
	names, cs := makeContainersN(10)
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	cmd := CommandRemove{Force: false, Link: "", Volumes: ""}
	for _, c := range cs {
		client.On("RemoveContainer", c, false, "", "").Return(nil)
	}
	err := Pumba{}.RemoveContainers(client, names, "", cmd)
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByNameRandom(t *testing.T) {
	// prepare test data and mocks
	names, cs := makeContainersN(10)
	client := container.NewMockSamalbaClient()
	cmd := CommandRemove{Force: false, Link: "mylink", Volumes: "myvol"}
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("RemoveContainer", mock.AnythingOfType("container.Container"), false, "mylink", "myvol").Return(nil)
	// do action
	RandomMode = true
	err := Pumba{}.RemoveContainers(client, names, "", cmd)
	RandomMode = false
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByPattern(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandRemove{Force: false, Link: "mylink", Volumes: "myvol"}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	for _, c := range cs {
		client.On("RemoveContainer", c, false, "mylink", "myvol").Return(nil)
	}
	// do action
	err := Pumba{}.RemoveContainers(client, []string{}, "^c", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestRemoveByPatternRandom(t *testing.T) {
	// prepare test data and mocks
	_, cs := makeContainersN(10)
	cmd := CommandRemove{Force: false, Link: "mylink", Volumes: "myvol"}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("RemoveContainer", mock.AnythingOfType("container.Container"), false, "mylink", "myvol").Return(nil)
	// do action
	RandomMode = true
	err := Pumba{}.RemoveContainers(client, []string{}, "^c", cmd)
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
	err := Pumba{}.PauseContainers(client, names, "", cmd)
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
	err := Pumba{}.PauseContainers(client, []string{}, "^c", cmd)
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
	err := Pumba{}.PauseContainers(client, names, "", cmd)
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
		Duration:     1 * time.Second,
		Amount:       120,
		Variation:    25,
		Correlation:  15,
	}
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	for _, c := range cs {
		client.On("DisruptContainer", c, "eth1", "delay 120ms 25ms 15%").Return(nil)
	}
	// do action
	err := Pumba{}.NetemDelayContainers(client, names, "", cmd)
	// asserts
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

/*
func TestNetemByNameRandom(t *testing.T) {
	names, cs := makeContainersN(10)
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("DisruptContainer", mock.AnythingOfType("container.Container"), "delay 1000ms").Return(nil)
	RandomMode = true
	err := Pumba{}.NetemContainers(client, names, "", "delay 1000ms")
	RandomMode = false
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemByPattern(t *testing.T) {
	_, cs := makeContainersN(10)
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	for i := range cs {
		client.On("DisruptContainer", cs[i], "delay 3000ms:172.19.0.3").Return(nil)
	}
	err := Pumba{}.NetemContainers(client, []string{}, "^c", "delay 3000ms:172.19.0.3")
	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestNetemByPatternRandom(t *testing.T) {
	_, cs := makeContainersN(10)
	client := container.NewMockSamalbaClient()
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("DisruptContainer", mock.AnythingOfType("container.Container"), "172.19.0.3").Return(nil)
	RandomMode = true
	err := Pumba{}.NetemContainers(client, []string{}, "^c", "172.19.0.3")
	RandomMode = false
	assert.NoError(t, err)
	client.AssertExpectations(t)
}
*/

func TestSelectRandomContainer(t *testing.T) {
	_, cs := makeContainersN(10)
	c1 := randomContainer(cs)
	c2 := randomContainer(cs)
	assert.NotNil(t, c1)
	assert.NotNil(t, c2)
	assert.NotEqual(t, c1.Name(), c2.Name())
}
