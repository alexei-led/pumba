package netem

import (
	"context"
	"errors"
	"testing"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/require"
)

func TestMatchAllPlanningFailureDoesNotInjectOrRevert(t *testing.T) {
	client := container.NewMockClient(t)
	req := &container.NetemRequest{Container: &container.Container{ContainerID: "id"}, MatchAll: true}
	want := errors.New("plan rejected")
	client.EXPECT().PlanNetemContainer(req).Return(nil, want).Once()
	report := &fakeReport{}
	require.ErrorIs(t, runNetem(context.Background(), client, req, report), want)
	require.Empty(t, report.calls)
}
