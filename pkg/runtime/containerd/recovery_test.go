package containerd

import (
	"context"
	"reflect"
	"syscall"
	"testing"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	containerd "github.com/containerd/containerd/v2/client"
	containers "github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/errdefs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRecoverySidecarLabelRevalidation(t *testing.T) {
	for _, key := range []string{ctr.SkipLabel, ctr.RunIDLabel, ctr.TargetLabel, ctr.KindLabel} {
		t.Run(key, func(t *testing.T) {
			api := NewMockapiClient(t)
			cntr := NewMockContainer(t)
			labels := ctr.SidecarLabels("old", "target", ctr.SidecarKindStress)
			labels[key] = "changed"
			api.EXPECT().LoadContainer(mock.Anything, "sidecar").Return(cntr, nil).Once()
			cntr.EXPECT().Info(mock.Anything).Return(containers.Container{Labels: labels}, nil).Once()
			cntr.EXPECT().ID().Return("sidecar").Once()
			client := &containerdClient{client: api}
			err := client.RemoveRecoverySidecar(context.Background(), ctr.SidecarIdentity{ID: "sidecar", TargetID: "target", RunID: "old", Kind: ctr.SidecarKindStress})
			require.ErrorContains(t, err, "changed")
		})
	}
}

func TestRecoverySidecarAlreadyAbsent(t *testing.T) {
	api := NewMockapiClient(t)
	api.EXPECT().LoadContainer(mock.Anything, "sidecar").Return(nil, errdefs.ErrNotFound).Once()
	require.NoError(t, (&containerdClient{client: api}).RemoveRecoverySidecar(context.Background(), ctr.SidecarIdentity{ID: "sidecar", TargetID: "target", RunID: "old", Kind: ctr.SidecarKindStress}))
}

func TestRecoverySidecarKillsTaskAndVerifiesAbsence(t *testing.T) {
	api := NewMockapiClient(t)
	cntr := NewMockContainer(t)
	task := NewMockTask(t)
	id := ctr.SidecarIdentity{ID: "sidecar", TargetID: "target", RunID: "old", Kind: ctr.SidecarKindStress}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api.EXPECT().LoadContainer(mock.Anything, id.ID).Return(cntr, nil).Once()
	cntr.EXPECT().Info(mock.Anything).Return(containers.Container{Labels: ctr.SidecarLabels(id.RunID, id.TargetID, id.Kind)}, nil).Once()
	cntr.EXPECT().ID().Return(id.ID).Once()
	cntr.EXPECT().Task(mock.Anything, mock.MatchedBy(func(attach cio.Attach) bool { return attach == nil })).Return(task, nil).Once()
	exits := make(chan containerd.ExitStatus, 1)
	exits <- *containerd.NewExitStatus(137, time.Now(), nil)
	task.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(exits), nil).Once()
	task.EXPECT().Kill(mock.Anything, syscall.SIGKILL).Run(func(op context.Context, _ syscall.Signal, _ ...containerd.KillOpts) {
		cancel()
		require.NoError(t, op.Err())
		_, ok := op.Deadline()
		require.True(t, ok)
	}).Return(nil).Once()
	task.EXPECT().Delete(mock.Anything).Return(nil, nil).Once()
	cntr.EXPECT().Delete(mock.Anything, mock.MatchedBy(func(option containerd.DeleteOpts) bool {
		return reflect.ValueOf(option).Pointer() == reflect.ValueOf(containerd.WithSnapshotCleanup).Pointer()
	})).Return(nil).Once()
	api.EXPECT().LoadContainer(mock.Anything, id.ID).Run(func(op context.Context, _ string) { require.NoError(t, op.Err()) }).Return(nil, errdefs.ErrNotFound).Once()
	require.NoError(t, (&containerdClient{client: api}).RemoveRecoverySidecar(ctx, id))
}
