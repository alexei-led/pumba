package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	tcplan "github.com/alexei-led/pumba/pkg/tc"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func execFrame(stream byte, payload string) []byte {
	header := []byte{stream, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
	return append(header, payload...)
}

func expectRawExec(api *mocks.APIClient, id string, argv []string, privileged bool, raw []byte, inspect ctypes.ExecInspect) {
	conn := &mockConn{}
	conn.On("Close").Return(nil)
	api.EXPECT().ContainerExecCreate(mock.Anything, id, ctypes.ExecOptions{
		AttachStdout: true, AttachStderr: true, Privileged: privileged, Cmd: argv,
	}).Return(ctypes.ExecCreateResponse{ID: "raw-exec"}, nil).Once()
	api.EXPECT().ContainerExecAttach(mock.Anything, "raw-exec", ctypes.ExecAttachOptions{}).Return(types.HijackedResponse{
		Conn: conn, Reader: bufio.NewReader(bytes.NewReader(raw)),
	}, nil).Once()
	api.EXPECT().ContainerExecInspect(mock.Anything, "raw-exec").Return(inspect, nil).Once()
}

func TestRunExecOutputFrames(t *testing.T) {
	stdout, stderr := execFrame(1, "out"), execFrame(2, "err")
	tests := []struct {
		name        string
		raw         []byte
		out, errOut string
	}{
		{name: "empty"},
		{name: "stdout", raw: stdout, out: "out"},
		{name: "stderr", raw: stderr, errOut: "err"},
		{name: "zero length stdout", raw: execFrame(1, "")},
		{name: "zero length stderr", raw: execFrame(2, "")},
		{name: "multiple frames", raw: bytes.Join([][]byte{stdout, stderr, execFrame(1, ""), stdout}, nil), out: "outout", errOut: "err"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine(t)
			expectRawExec(api, "target", []string{"probe"}, false, tt.raw, ctypes.ExecInspect{})
			client := dockerClient{containerAPI: api}
			out, errOut, err := client.runExec(context.Background(), "target", []string{"probe"}, false)
			require.NoError(t, err)
			assert.Equal(t, tt.out, out)
			assert.Equal(t, tt.errOut, errOut)
		})
	}
}

func TestRunExecRejectsTruncatedFrames(t *testing.T) {
	for _, stream := range []byte{1, 2} {
		fault := execFrame(stream, qdiscShowNetem)
		for _, prefix := range [][]byte{nil, execFrame(1, "complete\n"), execFrame(2, "warning\n")} {
			for cut := 1; cut < len(fault); cut++ {
				t.Run(fmt.Sprintf("stream%d/prefix%d/cut%d", stream, len(prefix), cut), func(t *testing.T) {
					api := NewMockEngine(t)
					expectRawExec(api, "target", []string{"probe"}, false, bytes.Join([][]byte{prefix, fault[:cut]}, nil), ctypes.ExecInspect{})
					client := dockerClient{containerAPI: api}
					out, errOut, err := client.runExec(context.Background(), "target", []string{"probe"}, false)
					require.ErrorContains(t, err, "demultiplex")
					assert.Empty(t, out)
					assert.Empty(t, errOut)
				})
			}
		}
	}
}

func TestRunExecRejectsUntrustedFrameLength(t *testing.T) {
	api := NewMockEngine(t)
	expectRawExec(api, "target", []string{"probe"}, false, []byte{1, 0, 0, 0, 255, 255, 255, 255}, ctypes.ExecInspect{})
	client := dockerClient{containerAPI: api}
	out, errOut, err := client.runExec(context.Background(), "target", []string{"probe"}, false)
	require.ErrorContains(t, err, "demultiplex")
	assert.Empty(t, out)
	assert.Empty(t, errOut)
}

func TestRunExecRejectsRunningExec(t *testing.T) {
	api := NewMockEngine(t)
	expectRawExec(api, "target", []string{"probe"}, false, execFrame(1, "out"), ctypes.ExecInspect{Running: true})
	client := dockerClient{containerAPI: api}
	out, errOut, err := client.runExec(context.Background(), "target", []string{"probe"}, false)
	require.ErrorContains(t, err, "still running")
	assert.Empty(t, out)
	assert.Empty(t, errOut)
}

func TestInspectNetnsRejectsIncompleteExecEvidence(t *testing.T) {
	for _, image := range []string{"", "nettools"} {
		for _, fault := range []string{"qdisc", "iptables", "backend warning", "running"} {
			t.Run(image+"/"+fault, func(t *testing.T) {
				api := NewMockEngine(t)
				id, privileged := "target", true
				if image != "" {
					id, privileged = "inspector", false
					expectSidecar(api, "target", id, ctr.SidecarLabels("status1", "target", ctr.SidecarKindInspect))
				}
				argv, raw := tcplan.QdiscShowCommand(), execFrame(1, qdiscShowNetem)
				inspect := ctypes.ExecInspect{}
				if fault == "iptables" || fault == "backend warning" {
					expectExec(api, id, privileged, execStep{argv: argv, stdout: "qdisc noqueue 0: dev lo root\n"})
					argv, raw = tcplan.IPTablesShowCommand(), execFrame(1, iptablesShowRule)
					if fault == "backend warning" {
						raw = append(execFrame(1, "-P INPUT ACCEPT\n"), execFrame(2, "# Warning: iptables-legacy tables present\n")...)
					}
				}
				if fault == "running" {
					inspect.Running = true // default ExitCode=0 is not completion evidence.
				} else {
					raw = raw[:len(raw)-1]
				}
				expectRawExec(api, id, argv, privileged, raw, inspect)
				client := dockerClient{containerAPI: api, imageAPI: api}
				state, err := client.InspectNetns(context.Background(), inspectReq(image))
				if fault == "running" {
					require.ErrorContains(t, err, "still running")
				} else {
					require.ErrorContains(t, err, "demultiplex")
				}
				assert.Nil(t, state)
			})
		}
	}
}
