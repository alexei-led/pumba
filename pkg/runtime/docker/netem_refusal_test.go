package docker

import (
	"context"
	"errors"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
	"github.com/alexei-led/pumba/pkg/tc"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNetemStartupRefusalStatus(t *testing.T) {
	for _, image := range []string{"", "nettools"} {
		for _, failure := range []string{"refused", "partial", "read", "running", "inspect", "cleanup", "partial cleanup", "read cleanup", "probe"} {
			cleanupFailure := failure == "cleanup" || failure == "partial cleanup" || failure == "read cleanup"
			if image == "" && cleanupFailure || image != "" && failure == "probe" {
				continue
			}
			t.Run(image+"/"+failure, func(t *testing.T) {
				req := &ctr.NetemRequest{Container: &ctr.Container{ContainerID: "target"}, Interface: "eth0", Command: []string{"delay", "100ms"}, Sidecar: ctr.SidecarSpec{Image: image}}
				argv, err := netns.NetemStart(req)
				require.NoError(t, err)
				api := NewMockEngine(t)
				cleanupErr := errors.New("remove failed")
				id, privileged := "target", true
				if image != "" {
					id, privileged = "sidecar", false
					expectSidecar(api, "target", id, ctr.SidecarLabels("", "target", ctr.SidecarKindNetem))
					if cleanupFailure {
						api.EXPECT().ContainerRemove(mock.Anything, id, ctypes.RemoveOptions{Force: true}).Unset()
						api.EXPECT().ContainerRemove(mock.Anything, id, ctypes.RemoveOptions{Force: true}).Return(cleanupErr).Once()
					}
				} else if failure != "probe" {
					expectExec(api, id, false, execStep{argv: []string{"which", "sh"}})
				}
				raw := execFrame(2, "refusing to replace a foreign or stale root qdisc")
				status := ctypes.ExecInspect{ExitCode: tc.NetemRefusedExitCode}
				switch failure {
				case "partial", "partial cleanup":
					status.ExitCode = 1
				case "read", "read cleanup":
					raw = raw[:len(raw)-1]
				case "running":
					status.Running = true
				case "probe":
					argv, privileged = []string{"which", "sh"}, false
				}
				expectRawExec(api, id, argv, privileged, raw, status)
				if failure == "inspect" {
					api.EXPECT().ContainerExecInspect(mock.Anything, "raw-exec").Unset()
					api.EXPECT().ContainerExecInspect(mock.Anything, "raw-exec").Return(ctypes.ExecInspect{}, errors.New("inspect failed")).Once()
				}
				client := dockerClient{containerAPI: api, imageAPI: api}
				err = client.NetemContainer(context.Background(), req)
				require.Error(t, err)
				require.Equal(t, failure == "refused" || failure == "cleanup", errors.Is(err, ctr.ErrNetemNotApplied))
				if cleanupFailure {
					require.ErrorIs(t, err, cleanupErr, "helper removal failure must remain visible")
				}
				if failure == "refused" || failure == "partial" || failure == "cleanup" {
					var exited *netns.ExecExitError
					require.ErrorAs(t, err, &exited)
					require.Equal(t, status.ExitCode, exited.Code)
					require.Equal(t, argv, exited.Argv)
					require.Contains(t, err.Error(), argv[2], "full script remains diagnostic, not classification evidence")
				}
			})
		}
	}
}

func TestStopNetemNeverMarksStartupRefusal(t *testing.T) {
	client, _ := newNetemTestClient(t, tc.NetemRefusedExitCode)
	err := client.StopNetemContainer(context.Background(), &ctr.NetemRequest{Container: &ctr.Container{ContainerID: "target"}, Interface: "eth0"})
	require.Error(t, err)
	require.NotErrorIs(t, err, ctr.ErrNetemNotApplied)
}
