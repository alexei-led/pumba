package containerd

import (
	"context"
	"errors"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	containers "github.com/containerd/containerd/v2/core/containers"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRemoveRecoverySidecarReturnsLoadAndInfoFailures(t *testing.T) {
	identity := ctr.SidecarIdentity{ID: "sidecar", TargetID: "target", RunID: "run", Kind: ctr.SidecarKindStress}
	loadErr := errors.New("container service unavailable")
	infoErr := errors.New("metadata unavailable")
	t.Run("load error", func(t *testing.T) {
		api := NewMockapiClient(t)
		api.EXPECT().LoadContainer(mock.Anything, identity.ID).Return(nil, loadErr).Once()
		err := (&containerdClient{client: api}).RemoveRecoverySidecar(context.Background(), identity)
		require.ErrorIs(t, err, loadErr)
	})
	t.Run("info error", func(t *testing.T) {
		api := NewMockapiClient(t)
		cntr := NewMockContainer(t)
		api.EXPECT().LoadContainer(mock.Anything, identity.ID).Return(cntr, nil).Once()
		cntr.EXPECT().Info(mock.Anything).Return(containers.Container{}, infoErr).Once()
		err := (&containerdClient{client: api}).RemoveRecoverySidecar(context.Background(), identity)
		require.ErrorIs(t, err, infoErr)
	})
}
