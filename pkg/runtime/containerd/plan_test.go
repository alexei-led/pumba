package containerd

import (
	"context"
	"net"
	"testing"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// executed is what one call of a containerdClient really ran: the sidecar
// image, if it created a sidecar, and the argv of every exec.
type executed struct {
	via      ctr.CommandVia
	image    string
	commands [][]string
}

// runRecorded runs fn against a client whose target "c1" (and, with a sidecar
// image, the sidecar) accepts any exec, and returns what fn executed.
func runRecorded(t *testing.T, sidecar bool, fn func(*containerdClient) error) executed {
	t.Helper()
	got := executed{via: ctr.ViaExec}
	record := func(_ context.Context, _ string, p *specs.Process, _ cio.Creator) (containerd.Process, error) {
		got.commands = append(got.commands, p.Args)
		return newSuccessProcess(t), nil
	}
	api := NewMockapiClient(t)
	targetTask := newRunningTask(t)
	if sidecar {
		targetTask.EXPECT().Pid().Return(42)
	} else {
		targetTask.EXPECT().Exec(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(record)
	}
	api.EXPECT().LoadContainer(mock.Anything, "c1").Return(newMockContainer(t, "c1", "nginx", nil, targetTask), nil)
	if sidecar {
		api.EXPECT().GetImage(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, ref string) (containerd.Image, error) {
			got.via, got.image = ctr.ViaSidecar, ref
			return NewMockImage(t), nil
		})
		sidecarTask := NewMockTask(t)
		sidecarTask.EXPECT().Start(mock.Anything).Return(nil)
		sidecarTask.EXPECT().Exec(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(record)
		exitCh := make(chan containerd.ExitStatus, 1)
		exitCh <- *containerd.NewExitStatus(137, time.Now(), nil)
		sidecarTask.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil)
		sidecarTask.EXPECT().Kill(mock.Anything, mock.Anything).Return(nil)
		sidecarTask.EXPECT().Delete(mock.Anything).Return(nil, nil)
		sc := NewMockContainer(t)
		sc.EXPECT().NewTask(mock.Anything, mock.Anything).Return(sidecarTask, nil)
		sc.EXPECT().Task(mock.Anything, mock.Anything).Return(sidecarTask, nil)
		sc.EXPECT().Delete(mock.Anything, mock.Anything).Return(nil)
		api.EXPECT().NewContainer(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(sc, nil)
	}
	require.NoError(t, fn(newTestClient(api)))
	return got
}

func cidr(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func TestPlanNetemContainerMatchesExecuted(t *testing.T) {
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
			IPs:    []*net.IPNet{cidr("10.0.0.0/8"), cidr("192.168.1.2/32")},
			SPorts: []string{"53"}, DPorts: []string{"80", "443"},
			Sidecar: ctr.SidecarSpec{Image: "nettools"}, RunID: "r1",
		}},
		{name: "scoped by exec", req: ctr.NetemRequest{
			Interface: "eth0", Command: []string{"rate", "1mbit"}, IPs: []*net.IPNet{cidr("10.0.0.1/32")}, RunID: "r1",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.req
			req.Container = testContainer("c1")
			sidecar := req.Sidecar.Image != ""

			plan, err := newTestClient(NewMockapiClient(t)).PlanNetemContainer(&req)
			require.NoError(t, err)

			apply := runRecorded(t, sidecar, func(c *containerdClient) error { return c.NetemContainer(context.Background(), &req) })
			revert := runRecorded(t, sidecar, func(c *containerdClient) error { return c.StopNetemContainer(context.Background(), &req) })

			assert.Equal(t, &ctr.CommandPlan{Via: apply.via, Image: apply.image, Apply: apply.commands, Revert: revert.commands}, plan)
		})
	}
}

func TestPlanIPTablesContainerMatchesExecuted(t *testing.T) {
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
			CmdSuffix: []string{"-j", "DROP"}, SrcIPs: []*net.IPNet{cidr("10.0.0.1/32")}, DstIPs: []*net.IPNet{cidr("10.0.0.0/24")},
			SPorts: []string{"53"}, DPorts: []string{"80"}, RunID: "r1",
		}},
		{name: "filtered tagged in sidecar", req: ctr.IPTablesRequest{
			CmdSuffix: []string{"-j", "DROP"}, SrcIPs: []*net.IPNet{cidr("10.0.0.1/32"), cidr("10.0.0.2/32")},
			Sidecar: ctr.SidecarSpec{Image: "nettools"}, RunID: "r1",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			add := tt.req
			add.Container = testContainer("c1")
			add.CmdPrefix = []string{"-I", "INPUT", "-i", "eth0"}
			del := add
			del.CmdPrefix = []string{"-D", "INPUT", "-i", "eth0"}
			sidecar := add.Sidecar.Image != ""

			plan, err := newTestClient(NewMockapiClient(t)).PlanIPTablesContainer(&add, &del)
			require.NoError(t, err)

			apply := runRecorded(t, sidecar, func(c *containerdClient) error { return c.IPTablesContainer(context.Background(), &add) })
			revert := runRecorded(t, sidecar, func(c *containerdClient) error { return c.StopIPTablesContainer(context.Background(), &del) })

			assert.Equal(t, &ctr.CommandPlan{Via: apply.via, Image: apply.image, Apply: apply.commands, Revert: revert.commands}, plan)
		})
	}
}
