package docker

import (
	"context"
	"errors"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRemoveRecoverySidecarRejectsIncompleteOrUnavailableMetadata(t *testing.T) {
	identity := ctr.SidecarIdentity{ID: "sidecar", TargetID: "target", RunID: "run", Kind: ctr.SidecarKindStress}
	inspectErr := errors.New("daemon unavailable")
	cases := []struct {
		name string
		info ctypes.InspectResponse
		err  error
		want string
	}{
		{name: "inspect error", err: inspectErr, want: "daemon unavailable"},
		{name: "missing base", info: ctypes.InspectResponse{Config: &ctypes.Config{}}, want: "incomplete sidecar metadata"},
		{name: "missing config", info: ctypes.InspectResponse{ContainerJSONBase: &ctypes.ContainerJSONBase{ID: identity.ID}}, want: "incomplete sidecar metadata"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := NewMockEngine(t)
			api.EXPECT().ContainerInspect(mock.Anything, identity.ID).Return(tc.info, tc.err).Once()
			err := (dockerClient{containerAPI: api}).RemoveRecoverySidecar(context.Background(), identity)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestRemoveRecoverySidecarReportsVerificationError(t *testing.T) {
	api := NewMockEngine(t)
	identity := ctr.SidecarIdentity{ID: "sidecar", TargetID: "target", RunID: "run", Kind: ctr.SidecarKindStress}
	info := ctypes.InspectResponse{
		ContainerJSONBase: &ctypes.ContainerJSONBase{ID: identity.ID},
		Config:            &ctypes.Config{Labels: ctr.SidecarLabels(identity.RunID, identity.TargetID, identity.Kind)},
	}
	verifyErr := errors.New("verification unavailable")
	api.EXPECT().ContainerInspect(mock.Anything, identity.ID).Return(info, nil).Once()
	api.EXPECT().ContainerRemove(mock.Anything, identity.ID, ctypes.RemoveOptions{Force: true}).Return(nil).Once()
	api.EXPECT().ContainerInspect(mock.Anything, identity.ID).Return(ctypes.InspectResponse{}, verifyErr).Once()
	err := (dockerClient{containerAPI: api}).RemoveRecoverySidecar(context.Background(), identity)
	require.ErrorIs(t, err, verifyErr)
	require.ErrorContains(t, err, "verify sidecar absence")
}
