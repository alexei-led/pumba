package iptables

import (
	"context"
	"errors"
	"testing"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/require"
)

func TestMatchAllPlanningFailureDoesNotInjectOrRevert(t *testing.T) {
	client := container.NewMockClient(t)
	add := &container.IPTablesRequest{Container: &container.Container{ContainerID: "id"}, CmdPrefix: []string{"-I", "INPUT"}, MatchAll: true}
	del := *add
	del.CmdPrefix = []string{"-D", "INPUT"}
	want := errors.New("plan rejected")
	client.EXPECT().PlanIPTablesContainer(add, &del).Return(nil, want).Once()
	report := &fakeReport{}
	require.ErrorIs(t, runIPTables(context.Background(), client, add, &del, report), want)
	require.Empty(t, report.calls)
}
