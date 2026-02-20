package chaos

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitLabels(t *testing.T) {
	tests := []struct {
		name     string
		raw      []string
		expected []string
	}{
		{"nil input", nil, nil},
		{"empty input", []string{}, nil},
		{"single label", []string{"k=v"}, []string{"k=v"}},
		{"multiple flags", []string{"k1=v1", "k2=v2"}, []string{"k1=v1", "k2=v2"}},
		{"comma separated", []string{"k1=v1,k2=v2"}, []string{"k1=v1", "k2=v2"}},
		{"mixed", []string{"k1=v1,k2=v2", "k3=v3"}, []string{"k1=v1", "k2=v2", "k3=v3"}},
		{"whitespace trimmed", []string{" k1=v1 , k2=v2 "}, []string{"k1=v1", "k2=v2"}},
		{"empty parts skipped", []string{"k1=v1,,k2=v2"}, []string{"k1=v1", "k2=v2"}},
		{"only commas", []string{",,"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitLabels(tt.raw)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// mockCommand implements Command for testing
type mockCommand struct {
	err   error
	calls int
}

func (m *mockCommand) Run(_ context.Context, _ bool) error {
	m.calls++
	return m.err
}

func TestRunChaosCommand_SingleRun(t *testing.T) {
	cmd := &mockCommand{}
	params := &GlobalParams{Interval: 0}

	err := RunChaosCommand(context.Background(), cmd, params)
	assert.NoError(t, err)
	assert.Equal(t, 1, cmd.calls)
}

func TestRunChaosCommand_Error(t *testing.T) {
	cmd := &mockCommand{err: errors.New("chaos failed")}
	params := &GlobalParams{Interval: 0}

	err := RunChaosCommand(context.Background(), cmd, params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chaos failed")
}

func TestRunChaosCommand_SkipErrors(t *testing.T) {
	cmd := &mockCommand{err: errors.New("chaos failed")}
	params := &GlobalParams{Interval: 0, SkipErrors: true}

	err := RunChaosCommand(context.Background(), cmd, params)
	assert.NoError(t, err)
}

func TestRunChaosCommand_ContextCancel(t *testing.T) {
	cmd := &mockCommand{}
	params := &GlobalParams{Interval: time.Hour} // long interval

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := RunChaosCommand(ctx, cmd, params)
	assert.NoError(t, err)
	assert.Equal(t, 1, cmd.calls)
}
