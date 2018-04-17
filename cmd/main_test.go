package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/action"
	"github.com/alexei-led/pumba/pkg/chaos/docker"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/urfave/cli"
)

//---- MOCK: Chaos Iterface

type ChaosMock struct {
	mock.Mock
}

func (m *ChaosMock) StopContainers(ctx context.Context, c container.Client, n []string, p string, cmd interface{}) error {
	args := m.Called(ctx, c, n, p, cmd)
	return args.Error(0)
}

func (m *ChaosMock) RemoveContainers(ctx context.Context, c container.Client, n []string, p string, cmd interface{}) error {
	args := m.Called(ctx, c, n, p, cmd)
	return args.Error(0)
}

func (m *ChaosMock) PauseContainers(ctx context.Context, c container.Client, n []string, p string, cmd interface{}) error {
	args := m.Called(ctx, c, n, p, cmd)
	return args.Error(0)
}

func (m *ChaosMock) NetemDelayContainers(ctx context.Context, c container.Client, n []string, p string, cmd interface{}) error {
	args := m.Called(ctx, c, n, p, cmd)
	return args.Error(0)
}

func (m *ChaosMock) NetemLossRandomContainers(ctx context.Context, c container.Client, n []string, p string, cmd interface{}) error {
	args := m.Called(ctx, c, n, p, cmd)
	return args.Error(0)
}

func (m *ChaosMock) NetemLossStateContainers(ctx context.Context, c container.Client, n []string, p string, cmd interface{}) error {
	args := m.Called(ctx, c, n, p, cmd)
	return args.Error(0)
}

func (m *ChaosMock) NetemLossGEmodelContainers(ctx context.Context, c container.Client, n []string, p string, cmd interface{}) error {
	args := m.Called(ctx, c, n, p, cmd)
	return args.Error(0)
}

func (m *ChaosMock) NetemRateContainers(ctx context.Context, c container.Client, n []string, p string, cmd interface{}) error {
	args := m.Called(ctx, c, n, p, cmd)
	return args.Error(0)
}

func (m *ChaosMock) StartContainers(ctx context.Context, c container.Client, n []string, p string, cmd interface{}) error {
	args := m.Called(ctx, c, n, p, cmd)
	return args.Error(0)
}

//---- TESTS

type mainTestSuite struct {
	suite.Suite
}

func (s *mainTestSuite) SetupSuite() {
	topContext = context.TODO()
}

func (s *mainTestSuite) TearDownSuite() {
}

func (s *mainTestSuite) SetupTest() {
}

func (s *mainTestSuite) TearDownTest() {
}

func (s *mainTestSuite) Test_main() {
	os.Args = []string{"pumba", "-v"}
	main()
}

func (s *mainTestSuite) Test_getNames() {
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.Parse([]string{"c1", "c2", "c3"})
	c := cli.NewContext(nil, globalSet, nil)
	names, pattern := getNamesOrPattern(c)
	assert.True(s.T(), len(names) == 3)
	assert.True(s.T(), pattern == "")
}

func (s *mainTestSuite) Test_getSingleName() {
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.Parse([]string{"single"})
	c := cli.NewContext(nil, globalSet, nil)
	names, pattern := getNamesOrPattern(c)
	assert.True(s.T(), len(names) == 1)
	assert.True(s.T(), names[0] == "single")
	assert.True(s.T(), pattern == "")
}

func (s *mainTestSuite) Test_getPattern() {
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.Parse([]string{"re2:^test"})
	c := cli.NewContext(nil, globalSet, nil)
	names, pattern := getNamesOrPattern(c)
	assert.True(s.T(), len(names) == 0)
	assert.True(s.T(), pattern == "^test")
}

func (s *mainTestSuite) Test_getPattern2() {
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.Parse([]string{"re2:(.+)test"})
	c := cli.NewContext(nil, globalSet, nil)
	names, pattern := getNamesOrPattern(c)
	assert.True(s.T(), len(names) == 0)
	assert.True(s.T(), pattern == "(.+)test")
}

func (s *mainTestSuite) Test_getIntervalValue_NoInterval() {
	// prepare
	set := flag.NewFlagSet("test", 0)
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.String("test", "me", "doc")
	parseErr := set.Parse([]string{})
	globalCtx := cli.NewContext(nil, globalSet, nil)
	c := cli.NewContext(nil, set, globalCtx)
	// invoke command
	interval, err := getIntervalValue(c)
	// asserts
	assert.NotEqual(s.T(), interval, 0)
	assert.NoError(s.T(), parseErr)
	assert.NoError(s.T(), err)
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
	interval, err := getIntervalValue(c)
	// asserts
	assert.NotEqual(s.T(), interval, 0)
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
	interval, err := getIntervalValue(c)
	names, pattern := getNamesOrPattern(c)
	// asserts
	assert.Equal(s.T(), interval, 10*time.Second)
	assert.NoError(s.T(), parseErr)
	assert.NoError(s.T(), err)
	assert.True(s.T(), len(names) == 0)
	assert.True(s.T(), pattern == "")
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
	interval, err := getIntervalValue(c)
	names, pattern := getNamesOrPattern(c)
	// asserts
	assert.Equal(s.T(), interval, 10*time.Second)
	assert.NoError(s.T(), parseErr)
	assert.NoError(s.T(), err)
	assert.True(s.T(), len(names) == 0)
	assert.True(s.T(), pattern == "^c")
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
	interval, err := getIntervalValue(c)
	names, pattern := getNamesOrPattern(c)
	// asserts
	assert.Equal(s.T(), interval, 10*time.Second)
	assert.NoError(s.T(), parseErr)
	assert.NoError(s.T(), err)
	assert.True(s.T(), len(names) == 2)
	assert.True(s.T(), pattern == "")
}

func (s *mainTestSuite) Test_handleSignals() {
	handleSignals()
}

func (s *mainTestSuite) Test_killSuccess() {
	// prepare
	set := flag.NewFlagSet("kill", 0)
	set.String("signal", "SIGTERM", "doc")
	c := cli.NewContext(nil, set, nil)
	// setup mock
	chaosMock := new(docker.MockChaosCommand)
	chaosMock.On("Run", mock.AnythingOfType("context.Context"), false).Return(nil)
	// invoke command
	err := kill(c)
	// asserts
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_pauseSuccess() {
	// prepare
	set := flag.NewFlagSet("pause", 0)
	set.String("duration", "10s", "doc")
	c := cli.NewContext(nil, set, nil)
	// setup mock
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	cmd := action.CommandPause{
		Duration: time.Duration(10 * time.Second),
	}
	chaosMock.On("PauseContainers", mock.Anything, nil, []string{}, "", cmd).Return(nil)
	// invoke command
	err := pause(c)
	// asserts
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_pauseMissingDuration() {
	// prepare
	set := flag.NewFlagSet("pause", 0)
	c := cli.NewContext(nil, set, nil)
	// invoke command
	err := pause(c)
	// asserts
	assert.EqualError(s.T(), err, "Undefined duration interval")
}

func (s *mainTestSuite) Test_pauseBadDuration() {
	// prepare
	set := flag.NewFlagSet("pause", 0)
	set.String("duration", "BAD", "doc")
	c := cli.NewContext(nil, set, nil)
	// invoke command
	err := pause(c)
	// asserts
	assert.EqualError(s.T(), err, "time: invalid duration BAD")
}

func (s *mainTestSuite) Test_stopSuccess() {
	// prepare
	set := flag.NewFlagSet("stop", 0)
	set.Int("time", 10, "doc")
	c := cli.NewContext(nil, set, nil)
	// setup mock
	cmd := action.CommandStop{WaitTime: 10}
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("StopContainers", mock.Anything, nil, []string{}, "", cmd).Return(nil)
	// invoke command
	err := stop(c)
	// asserts
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_stopError() {
	// prepare
	set := flag.NewFlagSet("stop", 0)
	set.Int("time", 10, "doc")
	c := cli.NewContext(nil, set, nil)
	// setup mock
	cmd := action.CommandStop{WaitTime: 10}
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("StopContainers", mock.Anything, nil, []string{}, "", cmd).Return(errors.New("ERROR"))
	// invoke command
	err := stop(c)
	// asserts
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_removeSuccess() {
	// prepare
	set := flag.NewFlagSet("stop", 0)
	set.Bool("force", true, "doc")
	set.Bool("links", true, "doc")
	set.Bool("volumes", true, "doc")
	c := cli.NewContext(nil, set, nil)
	// setup mock
	cmd := action.CommandRemove{Force: true, Links: true, Volumes: true}
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("RemoveContainers", mock.Anything, nil, []string{}, "", cmd).Return(nil)
	// invoke command
	err := remove(c)
	// asserts
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_netemDelaySuccess() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("duration", "10ms", "doc")
	netemSet.String("interface", "test0", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// delay flags
	delaySet := flag.NewFlagSet("delay", 0)
	delaySet.Int("time", 200, "doc")
	delaySet.Int("jitter", 20, "doc")
	delaySet.Float64("correlation", 1.5, "doc")
	delaySet.String("distribution", "normal", "doc")
	delaySet.Parse([]string{"c1", "c2", "c3"})
	delayCtx := cli.NewContext(nil, delaySet, netemCtx)
	// setup mock
	cmd := action.CommandNetemDelay{
		NetInterface: "test0",
		Duration:     10 * time.Millisecond,
		Time:         200,
		Jitter:       20,
		Correlation:  1.5,
		Distribution: "normal",
	}
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("NetemDelayContainers", mock.Anything, nil, []string{"c1", "c2", "c3"}, "", cmd).Return(nil)
	// invoke command
	err := netemDelay(delayCtx)
	// asserts
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_netemDelayNoDuration() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("interface", "test0", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// delay flags
	delaySet := flag.NewFlagSet("delay", 0)
	delaySet.Int("Time", 200, "doc")
	delaySet.Int("jitter", 20, "doc")
	delaySet.Float64("correlation", 10, "doc")
	delaySet.Parse([]string{"c1", "c2", "c3"})
	delayCtx := cli.NewContext(nil, delaySet, netemCtx)
	// invoke command
	err := netemDelay(delayCtx)
	// asserts
	assert.EqualError(s.T(), err, "Undefined duration interval")
}

func (s *mainTestSuite) Test_netemDelayBadDuration() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("interface", "test0", "doc")
	netemSet.String("duration", "BAD", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// delay flags
	delaySet := flag.NewFlagSet("delay", 0)
	delaySet.Int("time", 200, "doc")
	delaySet.Int("jitter", 20, "doc")
	delaySet.Float64("correlation", 10, "doc")
	delaySet.Parse([]string{"c1", "c2", "c3"})
	delayCtx := cli.NewContext(nil, delaySet, netemCtx)
	// invoke command
	err := netemDelay(delayCtx)
	// asserts
	assert.EqualError(s.T(), err, "time: invalid duration BAD")
}

func (s *mainTestSuite) Test_netemDelayBigDuration() {
	// set global interval to 5s
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.String("interval", "5s", "doc")
	globalCtx := cli.NewContext(nil, globalSet, nil)
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("interface", "test0", "doc")
	netemSet.String("duration", "10s", "doc")
	netemCtx := cli.NewContext(nil, netemSet, globalCtx)
	// delay flags
	delaySet := flag.NewFlagSet("delay", 0)
	delaySet.Int("time", 200, "doc")
	delaySet.Int("jitter", 20, "doc")
	delaySet.Float64("correlation", 10, "doc")
	delaySet.Parse([]string{"c1", "c2", "c3"})
	delayCtx := cli.NewContext(nil, delaySet, netemCtx)
	// invoke command
	err := netemDelay(delayCtx)
	// asserts
	assert.EqualError(s.T(), err, "Duration cannot be bigger than interval")
}

func (s *mainTestSuite) Test_netemDelayBadNetInterface() {
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("interface", "hello test", "doc")
	netemSet.String("duration", "10ms", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// delay flags
	delaySet := flag.NewFlagSet("delay", 0)
	delaySet.Int("time", 200, "doc")
	delaySet.Int("jitter", 20, "doc")
	delaySet.Float64("correlation", 10, "doc")
	delaySet.Parse([]string{"c1", "c2", "c3"})
	delayCtx := cli.NewContext(nil, delaySet, netemCtx)
	// invoke command
	err := netemDelay(delayCtx)
	// asserts
	assert.EqualError(s.T(), err, "Bad network interface name. Must match '[a-zA-Z]+[0-9]{0,2}'")
}

func (s *mainTestSuite) Test_netemDelayInvalidJitter() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("interface", "test0", "doc")
	netemSet.String("duration", "10ms", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// delay flags
	delaySet := flag.NewFlagSet("delay", 0)
	delaySet.Int("time", 200, "doc")
	delaySet.Int("jitter", -10, "doc")
	delaySet.Float64("correlation", 10, "doc")
	delaySet.Parse([]string{"c1", "c2", "c3"})
	delayCtx := cli.NewContext(nil, delaySet, netemCtx)
	// invoke command
	err := netemDelay(delayCtx)
	// asserts
	assert.EqualError(s.T(), err, "Invalid delay jitter")
}

func (s *mainTestSuite) Test_netemDelayInvalidTime() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("interface", "test0", "doc")
	netemSet.String("duration", "10ms", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// delay flags
	delaySet := flag.NewFlagSet("delay", 0)
	delaySet.Int("time", -20, "doc")
	delaySet.Int("jitter", 20, "doc")
	delaySet.Float64("correlation", 101.5, "doc")
	delaySet.Parse([]string{"c1", "c2", "c3"})
	delayCtx := cli.NewContext(nil, delaySet, netemCtx)
	// invoke command
	err := netemDelay(delayCtx)
	// asserts
	assert.EqualError(s.T(), err, "Invalid delay time")
}

func (s *mainTestSuite) Test_netemDelayInvalidCorrelation() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("interface", "test0", "doc")
	netemSet.String("duration", "10ms", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// delay flags
	delaySet := flag.NewFlagSet("delay", 0)
	delaySet.Int("time", 200, "doc")
	delaySet.Int("jitter", 20, "doc")
	delaySet.Float64("correlation", 101.5, "doc")
	delaySet.Parse([]string{"c1", "c2", "c3"})
	delayCtx := cli.NewContext(nil, delaySet, netemCtx)
	// invoke command
	err := netemDelay(delayCtx)
	// asserts
	assert.EqualError(s.T(), err, "Invalid delay correlation: must be between 0.0 and 100.0")
}

func (s *mainTestSuite) Test_netemDelayInvalidDistribution() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("interface", "test0", "doc")
	netemSet.String("duration", "10ms", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// delay flags
	delaySet := flag.NewFlagSet("delay", 0)
	delaySet.Int("time", 200, "doc")
	delaySet.Int("jitter", 10, "doc")
	delaySet.Float64("correlation", 10, "doc")
	delaySet.String("distribution", "INVALID", "doc")
	delaySet.Parse([]string{"c1", "c2", "c3"})
	delayCtx := cli.NewContext(nil, delaySet, netemCtx)
	// invoke command
	err := netemDelay(delayCtx)
	// asserts
	assert.EqualError(s.T(), err, "Invalid delay distribution: must be one of {uniform | normal | pareto |  paretonormal}")
}

func (s *mainTestSuite) Test_netemLossRandomSuccess() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("duration", "10ms", "doc")
	netemSet.String("interface", "test0", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// delay flags
	delaySet := flag.NewFlagSet("loss", 0)
	delaySet.Float64("percent", 20, "doc")
	delaySet.Float64("correlation", 1.5, "doc")
	delaySet.Parse([]string{"c1", "c2", "c3"})
	delayCtx := cli.NewContext(nil, delaySet, netemCtx)
	// setup mock
	cmd := action.CommandNetemLossRandom{
		NetInterface: "test0",
		Duration:     10 * time.Millisecond,
		Percent:      20.0,
		Correlation:  1.5,
	}
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("NetemLossRandomContainers", mock.Anything, nil, []string{"c1", "c2", "c3"}, "", cmd).Return(nil)
	// invoke command
	err := netemLossRandom(delayCtx)
	// asserts
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_netemLossStateSuccess() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("duration", "10ms", "doc")
	netemSet.String("interface", "test0", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// delay flags
	delaySet := flag.NewFlagSet("loss-state", 0)
	delaySet.Float64("p13", 17.5, "doc")
	delaySet.Float64("p31", 79.26, "doc")
	delaySet.Float64("p32", 1.5, "doc")
	delaySet.Float64("p23", 7.5, "doc")
	delaySet.Float64("p14", 9.31, "doc")
	delaySet.Parse([]string{"c1", "c2", "c3"})
	delayCtx := cli.NewContext(nil, delaySet, netemCtx)
	// setup mock
	cmd := action.CommandNetemLossState{
		NetInterface: "test0",
		Duration:     10 * time.Millisecond,
		P13:          17.5,
		P31:          79.26,
		P32:          1.5,
		P23:          7.5,
		P14:          9.31,
	}
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("NetemLossStateContainers", mock.Anything, nil, []string{"c1", "c2", "c3"}, "", cmd).Return(nil)
	// invoke command
	err := netemLossState(delayCtx)
	// asserts
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_netemLossGEmodelSuccess() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("duration", "10ms", "doc")
	netemSet.String("interface", "test0", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// delay flags
	delaySet := flag.NewFlagSet("loss-state", 0)
	delaySet.Float64("pg", 7.5, "doc")
	delaySet.Float64("pb", 92.1, "doc")
	delaySet.Float64("one-h", 82.34, "doc")
	delaySet.Float64("one-k", 8.32, "doc")
	delaySet.Parse([]string{"c1", "c2", "c3"})
	delayCtx := cli.NewContext(nil, delaySet, netemCtx)
	// setup mock
	cmd := action.CommandNetemLossGEmodel{
		NetInterface: "test0",
		Duration:     10 * time.Millisecond,
		PG:           7.5,
		PB:           92.1,
		OneH:         82.34,
		OneK:         8.32,
	}
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("NetemLossGEmodelContainers", mock.Anything, nil, []string{"c1", "c2", "c3"}, "", cmd).Return(nil)
	// invoke command
	err := netemLossGEmodel(delayCtx)
	// asserts
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_netemRateSuccess() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("duration", "10ms", "doc")
	netemSet.String("interface", "test0", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// rate flags
	rateSet := flag.NewFlagSet("rate", 0)
	rateSet.String("rate", "300kbit", "doc")
	rateSet.Int("packetoverhead", 10, "doc")
	rateSet.Int("cellsize", 20, "doc")
	rateSet.Int("celloverhead", 30, "doc")
	rateSet.Parse([]string{"c1", "c2", "c3"})
	rateCtx := cli.NewContext(nil, rateSet, netemCtx)
	// setup mock
	cmd := action.CommandNetemRate{
		NetInterface:   "test0",
		Duration:       10 * time.Millisecond,
		Rate:           "300kbit",
		PacketOverhead: 10,
		CellSize:       20,
		CellOverhead:   30,
	}
	chaosMock := &ChaosMock{}
	chaos = chaosMock
	chaosMock.On("NetemRateContainers", mock.Anything, nil, []string{"c1", "c2", "c3"}, "", cmd).Return(nil)
	// invoke command
	err := netemRate(rateCtx)
	// asserts
	assert.NoError(s.T(), err)
	chaosMock.AssertExpectations(s.T())
}

func (s *mainTestSuite) Test_netemRateInvalidRate() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("duration", "10ms", "doc")
	netemSet.String("interface", "test0", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// rate flags
	rateSet := flag.NewFlagSet("rate", 0)
	rateSet.String("rate", "300", "doc")
	rateSet.Int("packetoverhead", 10, "doc")
	rateSet.Int("cellsize", 20, "doc")
	rateSet.Int("celloverhead", 30, "doc")
	rateSet.Parse([]string{"c1", "c2", "c3"})
	rateCtx := cli.NewContext(nil, rateSet, netemCtx)
	// invoke command
	err := netemRate(rateCtx)
	// asserts
	assert.EqualError(s.T(), err, "Invalid rate. Must match '[0-9]+[gmk]?bit'")
}

func (s *mainTestSuite) Test_netemRateEmptyRate() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("duration", "10ms", "doc")
	netemSet.String("interface", "test0", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// rate flags
	rateSet := flag.NewFlagSet("rate", 0)
	rateSet.String("rate", "", "doc")
	rateSet.Int("packetoverhead", 10, "doc")
	rateSet.Int("cellsize", -20, "doc")
	rateSet.Int("celloverhead", 30, "doc")
	rateSet.Parse([]string{"c1", "c2", "c3"})
	rateCtx := cli.NewContext(nil, rateSet, netemCtx)
	// invoke command
	err := netemRate(rateCtx)
	// asserts
	assert.EqualError(s.T(), err, "Undefined rate limit")
}

func (s *mainTestSuite) Test_netemRateInvalidCellSize() {
	// prepare test data
	// netem flags
	netemSet := flag.NewFlagSet("netem", 0)
	netemSet.String("duration", "10ms", "doc")
	netemSet.String("interface", "test0", "doc")
	netemCtx := cli.NewContext(nil, netemSet, nil)
	// rate flags
	rateSet := flag.NewFlagSet("rate", 0)
	rateSet.String("rate", "300kbit", "doc")
	rateSet.Int("packetoverhead", 10, "doc")
	rateSet.Int("cellsize", -20, "doc")
	rateSet.Int("celloverhead", 30, "doc")
	rateSet.Parse([]string{"c1", "c2", "c3"})
	rateCtx := cli.NewContext(nil, rateSet, netemCtx)
	// invoke command
	err := netemRate(rateCtx)
	// asserts
	assert.EqualError(s.T(), err, "Invalid cell size: must be a non-negative integer")
}

func TestMainTestSuite(t *testing.T) {
	suite.Run(t, new(mainTestSuite))
}
