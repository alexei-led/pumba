package docker

import (
	"context"
	"errors"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/containerd/errdefs"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRemoveRecoverySidecar(t *testing.T) {
	id := ctr.SidecarIdentity{ID: "sidecar", TargetID: "original", RunID: "old", Kind: ctr.SidecarKindStress}
	for _, tt := range []struct {
		name                          string
		change                        string
		absent, remains, removeFailed bool
		wantErr                       string
	}{
		{name: "remove verified"}, {name: "already absent", absent: true},
		{name: "wrong skip", change: ctr.SkipLabel, wantErr: "changed"},
		{name: "wrong run", change: ctr.RunIDLabel, wantErr: "changed"},
		{name: "wrong target", change: ctr.TargetLabel, wantErr: "changed"},
		{name: "wrong kind", change: ctr.KindLabel, wantErr: "changed"},
		{name: "wrong exact id", change: "id", wantErr: "changed"},
		{name: "remains", remains: true, wantErr: "still exists"},
		{name: "remove failure", removeFailed: true, remains: true, wantErr: "remove recovery"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine(t)
			labels := ctr.SidecarLabels(id.RunID, id.TargetID, id.Kind)
			info := ctypes.InspectResponse{ContainerJSONBase: &ctypes.ContainerJSONBase{ID: id.ID}, Config: &ctypes.Config{Labels: labels}}
			if tt.change == "id" {
				info.ID = "different"
			} else if tt.change != "" {
				labels[tt.change] = "changed"
			}
			missing := errdefs.ErrNotFound
			if tt.absent {
				api.EXPECT().ContainerInspect(mock.Anything, id.ID).Return(ctypes.InspectResponse{}, missing).Once()
			} else {
				api.EXPECT().ContainerInspect(mock.Anything, id.ID).Return(info, nil).Once()
				if tt.change == "" {
					var removeErr error
					if tt.removeFailed {
						removeErr = errors.New("denied")
					}
					api.EXPECT().ContainerRemove(mock.Anything, id.ID, ctypes.RemoveOptions{Force: true}).Run(func(ctx context.Context, _ string, _ ctypes.RemoveOptions) {
						_, ok := ctx.Deadline()
						require.True(t, ok)
					}).Return(removeErr).Once()
					if tt.remains {
						api.EXPECT().ContainerInspect(mock.Anything, id.ID).Return(info, nil).Once()
					} else {
						api.EXPECT().ContainerInspect(mock.Anything, id.ID).Return(ctypes.InspectResponse{}, missing).Once()
					}
				}
			}
			err := (dockerClient{containerAPI: api}).RemoveRecoverySidecar(context.Background(), id)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestRemoveRecoverySidecarCanceled(t *testing.T) {
	api := NewMockEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := (dockerClient{containerAPI: api}).RemoveRecoverySidecar(ctx, ctr.SidecarIdentity{ID: "s", TargetID: "t", RunID: "old", Kind: ctr.SidecarKindNetem})
	require.ErrorIs(t, err, context.Canceled)
}
