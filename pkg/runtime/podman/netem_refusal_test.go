package podman

import (
	"context"
	"fmt"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNetemRootfulPreservesRefusal(t *testing.T) {
	delegate := ctr.NewMockClient(t)
	client := &podmanClient{Client: delegate}
	req := &ctr.NetemRequest{Container: &ctr.Container{ContainerID: "target"}, Interface: "eth0"}
	delegate.EXPECT().NetemContainer(mock.Anything, req).Return(fmt.Errorf("startup: %w", ctr.ErrNetemNotApplied)).Once()
	require.ErrorIs(t, client.NetemContainer(context.Background(), req), ctr.ErrNetemNotApplied)
}
