package main

import (
	"errors"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/gaia-adm/pumba/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/urfave/cli"
)

//---- MOCK: Chaos Iterface

type ChaosMock struct {
	mock.Mock
}

func (m *ChaosMock) StopContainers(c container.Client, n []string, p string, t int) error {
	args := m.Called(c, n, p, t)
	return args.Error(0)
}

func (m *ChaosMock) KillContainers(c container.Client, n []string, p string, s string) error {
	args := m.Called(c, n, p, s)
	return args.Error(0)
}

func (m *ChaosMock) RemoveContainers(c container.Client, n []string, p string, f bool, l string, v string) error {
	args := m.Called(c, n, p, f, l, v)
	return args.Error(0)
}

func (m *ChaosMock) PauseContainers(c container.Client, n []string, p string, d time.Duration) error {
	args := m.Called(c, n, p, d)
	return args.Error(0)
}

func (m *ChaosMock) NetemContainers(c container.Client, n []string, p string, cmd string) error {
	args := m.Called(c, n, p, cmd)
	return args.Error(0)
}

//---- TESTS

type mainTestSuite struct {
	suite.Suite
}

func (s *mainTestSuite) SetupSuite() {
	testRun = true
}

func (s *mainTestSuite) TearDownSuite() {
}

func (s *mainTestSuite) SetupTest() {
	containerNames = []string{}
	containerPattern = ""
}

func (s *mainTestSuite) TearDownTest() {
}

func (s *mainTestSuite) Test_main() {
	os.Args = []string{"pumba", "-v"}
	main()
}

func (s *mainTestSuite) Test_beforeCommand_NoInterval() {
	// prepare
	set := flag.NewFlagSet("test", 0)
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.String("test", "me", "doc")
	parseErr := set.Parse([]string{})
	globalCtx := cli.NewContext(nil, globalSet, nil)
	c := cli.NewContext(nil, set, globalCtx)
	// invoke command
	err := beforeCommand(c)
	// asserts
	assert.NoError(s.T(), parseErr)
	assert.Error(s.T(), err)
	assert.EqualError(s.T(), err, "Undefined interval value.")
}

func (s *mainTestSuite) Test_beforeCommand_BadInterval() {
	// prepare
	set := flag.NewFlagSet("test", 0)
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.String("interval", "BAD", "doc")
	parseErr := set.Parse([]string{})
	globalCtx := cli.NewContext(nil, globalSet, nil)
	c := cli.NewContext(nil, set, globalCtx)
	// invoke command
	err := beforeCommand(c)
	// asserts
	assert.NoError(s.T(), parseErr)
	assert.Error(s.T(), err)
	assert.EqualError(s.T(), err, "time: invalid duration BAD")
}

func (s *mainTestSuite) Test_beforeCommand_EmptyArgs() {
	// prepare
	set := flag.NewFlagSet("test", 0)
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.String("interval", "10s", "doc")
	parseErr := set.Parse([]string{})
	globalCtx := cli.NewContext(nil, globalSet, nil)
	c := cli.NewContext(nil, set, globalCtx)
	// invoke command
	err := beforeCommand(c)
	// asserts
	assert.NoError(s.T(), parseErr)
	assert.NoError(s.T(), err)
	assert.True(s.T(), len(containerNames) == 0)
}

func (s *mainTestSuite) Test_beforeCommand_Re2Args() {
	// prepare
	set := flag.NewFlagSet("test", 0)
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.String("interval", "10s", "doc")
	parseErr := set.Parse([]string{"re2:^c"})
	globalCtx := cli.NewContext(nil, globalSet, nil)
	c := cli.NewContext(nil, set, globalCtx)
	// invoke command
	err := beforeCommand(c)
	// asserts
	assert.NoError(s.T(), parseErr)
	assert.NoError(s.T(), err)
	assert.True(s.T(), containerPattern == "^c")
}

func (s *mainTestSuite) Test_beforeCommand_2Args() {
	// prepare
	set := flag.NewFlagSet("test", 0)
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.String("interval", "10s", "doc")
	parseErr := set.Parse([]string{"c1", "c2"})
	globalCtx := cli.NewContext(nil, globalSet, nil)
	c := cli.NewContext(nil, set, globalCtx)
	// invoke command
	err := beforeCommand(c)
	// asserts
	assert.NoError(s.T(), parseErr)
	assert.NoError(s.T(), err)
	assert.True(s.T(), len(containerNames) == 2)
}

func (s *mainTestSuite) Test_handleSignals() {
	wg.Add(1)
	handleSignals()
	wg.Done()
}

func (s *mainTestSuite) Test_killSucess() {
	// prepare
	set := flag.NewFlagSet("kill", 0)
	set.String("signal", "SIGTERM", "doc")
	c := cli.NewContext(nil, set, nil)
	timer := time.NewTimer(1 * time.Millisecond)
	commandTimeChan = timer.C
	// setup mock
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("KillContainers", nil, []string{}, "", "SIGTERM").Return(nil)
	// invoke command
	err := kill(c)
	// asserts
	// (!)WAIT till called action is completed (Sleep > Timer), it's executed in separate go routine
	time.Sleep(2 * time.Millisecond)
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_killBadSignal() {
	// prepare
	set := flag.NewFlagSet("kill", 0)
	set.String("signal", "UNKNOWN", "doc")
	c := cli.NewContext(nil, set, nil)
	// invoke command
	err := kill(c)
	// asserts
	assert.EqualError(s.T(), err, "Unexpected signal: UNKNOWN")
}

func (s *mainTestSuite) Test_killError() {
	// prepare
	set := flag.NewFlagSet("kill", 0)
	set.String("signal", "SIGTERM", "doc")
	c := cli.NewContext(nil, set, nil)
	timer := time.NewTimer(1 * time.Millisecond)
	commandTimeChan = timer.C
	// setup mock
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("KillContainers", nil, []string{}, "", "SIGTERM").Return(errors.New("ERROR"))
	// invoke command
	err := kill(c)
	// asserts
	// (!)WAIT till called action is completed (Sleep > Timer), it's executed in separate go routine
	time.Sleep(2 * time.Millisecond)
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_pauseSucess() {
	// prepare
	set := flag.NewFlagSet("pause", 0)
	set.String("duration", "10s", "doc")
	c := cli.NewContext(nil, set, nil)
	timer := time.NewTimer(1 * time.Millisecond)
	commandTimeChan = timer.C
	// setup mock
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("PauseContainers", nil, []string{}, "", time.Duration(10*time.Second)).Return(nil)
	// invoke command
	err := pause(c)
	// asserts
	// (!)WAIT till called action is completed (Sleep > Timer), it's executed in separate go routine
	time.Sleep(2 * time.Millisecond)
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_pauseMissingDuraation() {
	// prepare
	set := flag.NewFlagSet("pause", 0)
	c := cli.NewContext(nil, set, nil)
	timer := time.NewTimer(1 * time.Millisecond)
	commandTimeChan = timer.C
	// invoke command
	err := pause(c)
	// asserts
	assert.EqualError(s.T(), err, "Undefined duration interval")
}

func (s *mainTestSuite) Test_pauseBadDuraation() {
	// prepare
	set := flag.NewFlagSet("pause", 0)
	set.String("duration", "BAD", "doc")
	c := cli.NewContext(nil, set, nil)
	timer := time.NewTimer(1 * time.Millisecond)
	commandTimeChan = timer.C
	// invoke command
	err := pause(c)
	// asserts
	assert.EqualError(s.T(), err, "time: invalid duration BAD")
}

func (s *mainTestSuite) Test_stopSucess() {
	// prepare
	set := flag.NewFlagSet("stop", 0)
	set.Int("time", 10, "doc")
	c := cli.NewContext(nil, set, nil)
	timer := time.NewTimer(1 * time.Millisecond)
	commandTimeChan = timer.C
	// setup mock
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("StopContainers", nil, []string{}, "", 10).Return(nil)
	// invoke command
	err := stop(c)
	// asserts
	// (!)WAIT till called action is completed (Sleep > Timer), it's executed in separate go routine
	time.Sleep(2 * time.Millisecond)
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_stopError() {
	// prepare
	set := flag.NewFlagSet("stop", 0)
	set.Int("time", 10, "doc")
	c := cli.NewContext(nil, set, nil)
	timer := time.NewTimer(1 * time.Millisecond)
	commandTimeChan = timer.C
	// setup mock
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("StopContainers", nil, []string{}, "", 10).Return(errors.New("ERROR"))
	// invoke command
	err := stop(c)
	// asserts
	// (!)WAIT till called action is completed (Sleep > Timer), it's executed in separate go routine
	time.Sleep(2 * time.Millisecond)
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_removeSucess() {
	// prepare
	set := flag.NewFlagSet("stop", 0)
	set.Bool("force", true, "doc")
	set.String("link", "mylink", "doc")
	set.String("volumes", "myvolume", "doc")
	c := cli.NewContext(nil, set, nil)
	timer := time.NewTimer(1 * time.Millisecond)
	commandTimeChan = timer.C
	// setup mock
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("RemoveContainers", nil, []string{}, "", true, "mylink", "myvolume").Return(nil)
	// invoke command
	err := remove(c)
	// asserts
	// (!)WAIT till called action is completed (Sleep > Timer), it's executed in separate go routine
	time.Sleep(2 * time.Millisecond)
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func TestMainTestSuite(t *testing.T) {
	suite.Run(t, new(mainTestSuite))
}
