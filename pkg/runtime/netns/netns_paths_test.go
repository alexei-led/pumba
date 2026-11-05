package netns

import (
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/require"
)

func TestPlansRejectInvalidMatchAllRequestsBeforeReturningCommands(t *testing.T) {
	netem := &ctr.NetemRequest{Interface: "eth0", Command: []string{"delay", "1ms"}, SPorts: []string{"70000"}, MatchAll: true}
	for _, call := range []struct {
		name string
		run  func() error
	}{
		{name: "netem start", run: func() error { _, err := NetemStart(netem); return err }},
		{name: "netem stop", run: func() error { _, err := NetemStop(netem); return err }},
		{name: "netem plan", run: func() error { _, err := NetemPlan(netem); return err }},
	} {
		t.Run(call.name, func(t *testing.T) { require.Error(t, call.run()) })
	}

	badAdd := &ctr.IPTablesRequest{CmdPrefix: []string{"-I", "INPUT"}, CmdSuffix: []string{"-j", "DROP"}, SPorts: []string{"70000"}, MatchAll: true}
	goodAdd := &ctr.IPTablesRequest{CmdPrefix: []string{"-I", "INPUT"}, CmdSuffix: []string{"-j", "DROP"}}
	_, err := IPTablesCommands(badAdd)
	require.Error(t, err)
	_, err = IPTablesPlan(badAdd, goodAdd)
	require.Error(t, err)
	_, err = IPTablesPlan(goodAdd, badAdd)
	require.Error(t, err)
}
