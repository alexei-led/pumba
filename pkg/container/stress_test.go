package container

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStressCleanupError_PreservesCause(t *testing.T) {
	failure := errors.New("remove failed")
	cleanup := &StressCleanupError{Err: failure}
	require.ErrorIs(t, cleanup, failure)
	require.Equal(t, failure, errors.Unwrap(cleanup))
	require.EqualError(t, cleanup, "stress cleanup failed: remove failed")
}
