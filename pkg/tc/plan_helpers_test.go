package tc

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func startArgs(t *testing.T, req *NetemRequest) []string {
	t.Helper()
	args, err := Start(req)
	require.NoError(t, err)
	return args
}

func stopArgs(t *testing.T, req *NetemRequest) []string {
	t.Helper()
	args, err := Stop(req)
	require.NoError(t, err)
	return args
}

func iptablesCommands(t *testing.T, req *IPTablesRequest) [][]string {
	t.Helper()
	args, err := IPTablesCommands(req)
	require.NoError(t, err)
	return args
}
