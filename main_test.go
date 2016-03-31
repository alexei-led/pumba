package main

import (
	"testing"

	"github.com/gaia-adm/pumba/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

//---- MOCK: Chaos Iterface

type ChaosMock struct {
	mock.Mock
}

func (m *ChaosMock) StopByName(c container.Client, names []string) error {
	args := m.Called(c, names)
	return args.Error(0)
}

func (m *ChaosMock) StopByPattern(c container.Client, p string) error {
	args := m.Called(c, p)
	return args.Error(0)
}

func (m *ChaosMock) KillByName(c container.Client, names []string, signal string) error {
	args := m.Called(c, names, signal)
	return args.Error(0)
}

func (m *ChaosMock) KillByPattern(c container.Client, p string, signal string) error {
	args := m.Called(c, p, signal)
	return args.Error(0)
}

func (m *ChaosMock) RemoveByName(c container.Client, names []string, f bool) error {
	args := m.Called(c, names, f)
	return args.Error(0)
}

func (m *ChaosMock) RemoveByPattern(c container.Client, p string, f bool) error {
	args := m.Called(c, p, f)
	return args.Error(0)
}

//---- TESTS

func TestCreateChaos_StopByName(t *testing.T) {
	cmd := "c1,c2|10ms|STOP"
	limit := 3

	chaos := &ChaosMock{}
	for i := 0; i < limit; i++ {
		chaos.On("StopByName", nil, []string{"c1", "c2"}).Return(nil)
	}

	err := createChaos(chaos, []string{cmd}, limit)

	assert.NoError(t, err)
	chaos.AssertExpectations(t)
}

func TestCreateChaos_StopByPattern(t *testing.T) {
	cmd := "re2:^c|10ms|STOP"
	limit := 3

	chaos := &ChaosMock{}
	for i := 0; i < limit; i++ {
		chaos.On("StopByPattern", nil, "^c").Return(nil)
	}

	err := createChaos(chaos, []string{cmd}, limit)

	assert.NoError(t, err)
	chaos.AssertExpectations(t)
}

func TestCreateChaos_KillByName(t *testing.T) {
	cmd := "c1,c2|10ms|KILL"
	limit := 3

	chaos := &ChaosMock{}
	for i := 0; i < limit; i++ {
		chaos.On("KillByName", nil, []string{"c1", "c2"}, "SIGKILL").Return(nil)
	}

	err := createChaos(chaos, []string{cmd}, limit)

	assert.NoError(t, err)
	chaos.AssertExpectations(t)
}

func TestCreateChaos_KillByNameSignal(t *testing.T) {
	cmd := "c1,c2|10ms|KILL:SIGTEST"
	limit := 3

	chaos := &ChaosMock{}
	for i := 0; i < limit; i++ {
		chaos.On("KillByName", nil, []string{"c1", "c2"}, "SIGTEST").Return(nil)
	}

	err := createChaos(chaos, []string{cmd}, limit)

	assert.NoError(t, err)
	chaos.AssertExpectations(t)
}

func TestCreateChaos_MultiKillByNameSignal(t *testing.T) {
	cmd1 := "c1,c2|10ms|KILL:SIGTEST"
	cmd2 := "c3,c4|10ms|STOP"
	limit := 3

	chaos := &ChaosMock{}
	for i := 0; i < limit; i++ {
		chaos.On("KillByName", nil, []string{"c1", "c2"}, "SIGTEST").Return(nil)
		chaos.On("StopByName", nil, []string{"c3", "c4"}).Return(nil)
	}

	err := createChaos(chaos, []string{cmd1, cmd2}, limit*2)

	assert.NoError(t, err)
	chaos.AssertExpectations(t)
}

func TestCreateChaos_KillByPatternSignal(t *testing.T) {
	cmd := "re2:.|10ms|KILL:SIGTEST"
	limit := 3

	chaos := &ChaosMock{}
	for i := 0; i < limit; i++ {
		chaos.On("KillByPattern", nil, ".", "SIGTEST").Return(nil)
	}

	err := createChaos(chaos, []string{cmd}, limit)

	assert.NoError(t, err)
	chaos.AssertExpectations(t)
}

func TestCreateChaos_RemoveByName(t *testing.T) {
	cmd := "cc1,cc2|10ms|RM"
	limit := 3

	chaos := &ChaosMock{}
	for i := 0; i < limit; i++ {
		chaos.On("RemoveByName", nil, []string{"cc1", "cc2"}, true).Return(nil)
	}

	err := createChaos(chaos, []string{cmd}, limit)

	assert.NoError(t, err)
	chaos.AssertExpectations(t)
}

func TestCreateChaos_RemoveByPattern(t *testing.T) {
	cmd := "re2:(abc)|10ms|RM"
	limit := 3

	chaos := &ChaosMock{}
	for i := 0; i < limit; i++ {
		chaos.On("RemoveByPattern", nil, "(abc)", true).Return(nil)
	}

	err := createChaos(chaos, []string{cmd}, limit)

	assert.NoError(t, err)
	chaos.AssertExpectations(t)
}

func TestCreateChaos_ErrorCommandFormat(t *testing.T) {
	cmd := "10ms|RM"
	chaos := &ChaosMock{}

	err := createChaos(chaos, []string{cmd}, 0)

	assert.Error(t, err)
	chaos.AssertExpectations(t)
}

func TestCreateChaos_ErrorDurationFormat(t *testing.T) {
	cmd := "abc|hello|RM"
	chaos := &ChaosMock{}

	err := createChaos(chaos, []string{cmd}, 0)

	assert.Error(t, err)
	chaos.AssertExpectations(t)
}

func TestCreateChaos_ErrorCommand(t *testing.T) {
	cmd := "c1|10s|TEST"
	chaos := &ChaosMock{}

	err := createChaos(chaos, []string{cmd}, 0)

	assert.Error(t, err)
	chaos.AssertExpectations(t)
}

func Test_HandleSignals(t *testing.T) {
	wg.Add(1)
	handleSignals()
	wg.Done()
}
