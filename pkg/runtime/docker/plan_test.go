package docker

import (
	"context"
	"net"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// execRecorder captures what a dockerClient really runs: the sidecar image,
// if a sidecar was created, and the argv of every exec except the `which`
// probe, which is not part of a plan.
type execRecorder struct {
	image    string
	commands [][]string
}

// executed returns what was run as a plan of one phase, and resets.
func (r *execRecorder) executed() (ctr.CommandVia, string, [][]string) {
	via := ctr.ViaExec
	if r.image != "" {
		via = ctr.ViaSidecar
	}
	image, commands := r.image, r.commands
	r.image, r.commands = "", nil
	return via, image, commands
}

func newRecordingClient(t *testing.T) (dockerClient, *execRecorder) {
	t.Helper()
	rec := &execRecorder{}
	api := NewMockEngine(t)
	api.EXPECT().ContainerCreate(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").
		RunAndReturn(func(_ context.Context, config *ctypes.Config, _ *ctypes.HostConfig, _ *network.NetworkingConfig, _ *specs.Platform, _ string) (ctypes.CreateResponse, error) {
			rec.image = config.Image
			return ctypes.CreateResponse{ID: "sidecar"}, nil
		}).Maybe()
	api.EXPECT().ContainerStart(mock.Anything, "sidecar", ctypes.StartOptions{}).Return(nil).Maybe()
	api.EXPECT().ContainerRemove(mock.Anything, "sidecar", ctypes.RemoveOptions{Force: true}).Return(nil).Maybe()
	api.EXPECT().ContainerExecCreate(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, options ctypes.ExecOptions) (ctypes.ExecCreateResponse, error) {
			if options.Cmd[0] != "which" {
				rec.commands = append(rec.commands, options.Cmd)
			}
			return ctypes.ExecCreateResponse{ID: "exec"}, nil
		})
	api.EXPECT().ContainerExecAttach(mock.Anything, "exec", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	api.EXPECT().ContainerExecInspect(mock.Anything, "exec").Return(ctypes.ExecInspect{}, nil)
	return dockerClient{containerAPI: api, imageAPI: api}, rec
}

func ipNet(cidr string) *net.IPNet {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return n
}

func TestPlanNetemContainerMatchesExecuted(t *testing.T) {
	target := &ctr.Container{ContainerID: "target", ContainerName: "web"}
	tests := []struct {
		name string
		req  ctr.NetemRequest
	}{
		{name: "unscoped by exec", req: ctr.NetemRequest{Interface: "eth0", Command: []string{"delay", "100ms"}}},
		{name: "unscoped in sidecar", req: ctr.NetemRequest{
			Interface: "eth0", Command: []string{"loss", "10%"}, Sidecar: ctr.SidecarSpec{Image: "nettools"}, RunID: "r1",
		}},
		{name: "scoped in sidecar", req: ctr.NetemRequest{
			Interface: "eth1", Command: []string{"delay", "100ms", "10ms"},
			IPs:    []*net.IPNet{ipNet("10.0.0.0/8"), ipNet("192.168.1.2/32")},
			SPorts: []string{"53"}, DPorts: []string{"80", "443"},
			Sidecar: ctr.SidecarSpec{Image: "nettools"}, RunID: "r1",
		}},
		{name: "scoped by exec", req: ctr.NetemRequest{
			Interface: "eth0", Command: []string{"rate", "1mbit"}, IPs: []*net.IPNet{ipNet("10.0.0.1/32")}, RunID: "r1",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, rec := newRecordingClient(t)
			req := tt.req
			req.Container = target

			plan, err := client.PlanNetemContainer(&req)
			require.NoError(t, err)

			require.NoError(t, client.NetemContainer(context.Background(), &req))
			via, image, apply := rec.executed()
			require.NoError(t, client.StopNetemContainer(context.Background(), &req))
			_, _, revert := rec.executed()

			assert.Equal(t, &ctr.CommandPlan{Via: via, Image: image, Apply: apply, Revert: revert}, plan)
		})
	}
}

func TestPlanIPTablesContainerMatchesExecuted(t *testing.T) {
	target := &ctr.Container{ContainerID: "target", ContainerName: "web"}
	tests := []struct {
		name string
		req  ctr.IPTablesRequest
	}{
		{name: "unfiltered untagged by exec", req: ctr.IPTablesRequest{
			CmdSuffix: []string{"-m", "statistic", "--mode", "random", "--probability", "0.20", "-j", "DROP"},
		}},
		{name: "unfiltered tagged in sidecar", req: ctr.IPTablesRequest{
			CmdSuffix: []string{"-j", "DROP"}, Sidecar: ctr.SidecarSpec{Image: "nettools"}, RunID: "r1",
		}},
		{name: "filtered tagged by exec", req: ctr.IPTablesRequest{
			CmdSuffix: []string{"-j", "DROP"}, SrcIPs: []*net.IPNet{ipNet("10.0.0.1/32")}, DstIPs: []*net.IPNet{ipNet("10.0.0.0/24")},
			SPorts: []string{"53"}, DPorts: []string{"80"}, RunID: "r1",
		}},
		{name: "filtered tagged in sidecar", req: ctr.IPTablesRequest{
			CmdSuffix: []string{"-j", "DROP"}, SrcIPs: []*net.IPNet{ipNet("10.0.0.1/32"), ipNet("10.0.0.2/32")},
			Sidecar: ctr.SidecarSpec{Image: "nettools"}, RunID: "r1",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, rec := newRecordingClient(t)
			add := tt.req
			add.Container = target
			add.CmdPrefix = []string{"-I", "INPUT", "-i", "eth0"}
			del := add
			del.CmdPrefix = []string{"-D", "INPUT", "-i", "eth0"}

			plan, err := client.PlanIPTablesContainer(&add, &del)
			require.NoError(t, err)

			require.NoError(t, client.IPTablesContainer(context.Background(), &add))
			via, image, apply := rec.executed()
			require.NoError(t, client.StopIPTablesContainer(context.Background(), &del))
			_, _, revert := rec.executed()

			assert.Equal(t, &ctr.CommandPlan{Via: via, Image: image, Apply: apply, Revert: revert}, plan)
		})
	}
}
