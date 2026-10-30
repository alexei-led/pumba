package iptables

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeReport records what runIPTables reports about its target, in order.
type fakeReport struct {
	calls     []string
	revertErr error
}

func (f *fakeReport) Applied() { f.calls = append(f.calls, "applied") }

func (f *fakeReport) Reverted(err error) {
	f.calls = append(f.calls, "reverted")
	f.revertErr = err
}

func Test_runIPTables(t *testing.T) {
	type errs struct {
		stopErr bool
	}
	type args struct {
		container    *container.Container
		addCmdPrefix []string
		delCmdPrefix []string
		cmdSuffix    []string
		dstIPs       []*net.IPNet
		srcIPs       []*net.IPNet
		sports       []string
		dports       []string
		duration     time.Duration
		image        string
		pull         bool
		dryRun       bool
	}
	tests := []struct {
		name    string
		args    args
		abort   bool
		errs    errs
		wantErr bool
	}{
		{
			name: "iptables with duration",
			args: args{
				container: &container.Container{
					ContainerName: "c1",
					Labels:        map[string]string{},
					Networks:      map[string]container.NetworkLink{},
				},
				addCmdPrefix: []string{"-A", "INPUT"},
				delCmdPrefix: []string{"-D", "INPUT"},
				cmdSuffix:    []string{"-m", "statistic"},
				dstIPs:       []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				srcIPs:       []*net.IPNet{{IP: net.IP{10, 10, 10, 1}}},
				sports:       []string{"44"},
				dports:       []string{"662"},
				duration:     time.Microsecond * 10,
				image:        "test/image",
			},
		},
		{
			name: "iptables with CIDR IP",
			args: args{
				container: &container.Container{
					ContainerName: "c1",
					Labels:        map[string]string{},
					Networks:      map[string]container.NetworkLink{},
				},
				addCmdPrefix: []string{"-A", "INPUT"},
				delCmdPrefix: []string{"-D", "INPUT"},
				cmdSuffix:    []string{"-m", "statistic"},
				dstIPs:       []*net.IPNet{{IP: net.IP{10, 10, 10, 10}, Mask: net.IPMask{0, 0, 255, 255}}},
				srcIPs:       []*net.IPNet{{IP: net.IP{10, 10, 10, 1}, Mask: net.IPMask{0, 0, 255, 255}}},
				duration:     time.Microsecond * 10,
				image:        "test/image",
			},
		},
		{
			name: "iptables with abort",
			args: args{
				container: &container.Container{
					ContainerName: "c1",
					Labels:        map[string]string{},
					Networks:      map[string]container.NetworkLink{},
				},
				addCmdPrefix: []string{"-A", "INPUT"},
				delCmdPrefix: []string{"-D", "INPUT"},
				cmdSuffix:    []string{"-m", "statistic"},
				dstIPs:       []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				duration:     time.Microsecond * 10,
				image:        "test/image",
			},
			abort: true,
		},
		{
			name: "iptables cleanup failure is returned",
			args: args{
				container: &container.Container{
					ContainerName: "c1",
					Labels:        map[string]string{},
					Networks:      map[string]container.NetworkLink{},
				},
				addCmdPrefix: []string{"-A", "INPUT"},
				delCmdPrefix: []string{"-D", "INPUT"},
				cmdSuffix:    []string{"-m", "statistic"},
				dstIPs:       []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				duration:     time.Microsecond * 10,
				image:        "test/image",
			},
			errs:    errs{stopErr: true},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := container.NewMockClient(t)
			ctx, cancel := context.WithCancel(context.TODO())
			defer cancel()

			addReq := &container.IPTablesRequest{
				Container: tt.args.container,
				CmdPrefix: tt.args.addCmdPrefix,
				CmdSuffix: tt.args.cmdSuffix,
				SrcIPs:    tt.args.srcIPs,
				DstIPs:    tt.args.dstIPs,
				SPorts:    tt.args.sports,
				DPorts:    tt.args.dports,
				Duration:  tt.args.duration,
				Sidecar:   container.SidecarSpec{Image: tt.args.image, Pull: tt.args.pull},
				DryRun:    tt.args.dryRun,
			}
			delReq := &container.IPTablesRequest{
				Container: tt.args.container,
				CmdPrefix: tt.args.delCmdPrefix,
				CmdSuffix: tt.args.cmdSuffix,
				SrcIPs:    tt.args.srcIPs,
				DstIPs:    tt.args.dstIPs,
				SPorts:    tt.args.sports,
				DPorts:    tt.args.dports,
				Duration:  tt.args.duration,
				Sidecar:   container.SidecarSpec{Image: tt.args.image, Pull: tt.args.pull},
				DryRun:    tt.args.dryRun,
			}
			mockClient.EXPECT().IPTablesContainer(ctx, addReq).Return(nil)

			stopErr := error(nil)
			if tt.errs.stopErr {
				stopErr = errors.New("test error")
			}
			mockClient.EXPECT().StopIPTablesContainer(mock.Anything, delReq).Return(stopErr)

			// abort case: cancel ctx before runIPTables so the ctx.Done() branch
			// is exercised; otherwise the stopCtx timeout branch wins.
			if tt.abort {
				cancel()
			}

			report := &fakeReport{}
			err := runIPTables(ctx, mockClient, addReq, delReq, report)
			if (err != nil) != tt.wantErr {
				t.Errorf("runIPTables() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				assert.ErrorIs(t, err, stopErr)
			}
			assert.Equal(t, []string{"applied", "reverted"}, report.calls)
			assert.Equal(t, stopErr, report.revertErr)
		})
	}
}

func Test_runIPTables_rollsBackOnAddFailure(t *testing.T) {
	addErr := errors.New("add rule failed")
	tests := []struct {
		name       string
		cleanupErr error
	}{
		{name: "cleanup succeeds"},
		{name: "cleanup fails", cleanupErr: errors.New("cleanup failed")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := container.NewMockClient(t)
			ctr := &container.Container{ContainerName: "c1", Labels: map[string]string{}, Networks: map[string]container.NetworkLink{}}
			addReq := &container.IPTablesRequest{
				Container: ctr,
				CmdPrefix: []string{"-I", "INPUT"},
				CmdSuffix: []string{"-m", "statistic"},
				SPorts:    []string{"49", "50"},
				Duration:  time.Minute,
			}
			delReq := &container.IPTablesRequest{
				Container: ctr,
				CmdPrefix: []string{"-D", "INPUT"},
				CmdSuffix: []string{"-m", "statistic"},
				SPorts:    []string{"49", "50"},
				Duration:  time.Minute,
			}
			mockClient.EXPECT().IPTablesContainer(mock.Anything, addReq).Return(addErr).Once()
			mockClient.EXPECT().StopIPTablesContainer(mock.Anything, delReq).Return(tt.cleanupErr).Once()

			report := &fakeReport{}
			err := runIPTables(context.Background(), mockClient, addReq, delReq, report)
			assert.EqualError(t, err, "iptables failed: add rule failed")
			assert.Empty(t, report.calls, "rollback of a partial install is neither applied nor a cleanup outcome")
		})
	}
}

func Test_runIPTables_rollbackIgnoresCallerCancel(t *testing.T) {
	mockClient := container.NewMockClient(t)
	ctr := &container.Container{ContainerName: "c1", Labels: map[string]string{}, Networks: map[string]container.NetworkLink{}}
	addReq := &container.IPTablesRequest{Container: ctr, CmdPrefix: []string{"-I", "INPUT"}, DPorts: []string{"80"}, Duration: time.Minute}
	delReq := &container.IPTablesRequest{Container: ctr, CmdPrefix: []string{"-D", "INPUT"}, DPorts: []string{"80"}, Duration: time.Minute}

	// SIGTERM mid-install cancels the caller's ctx; rollback must still get a live, bounded ctx
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mockClient.EXPECT().IPTablesContainer(mock.Anything, addReq).Return(errors.New("add rule failed")).Once()
	mockClient.EXPECT().StopIPTablesContainer(mock.Anything, delReq).Run(func(cleanupCtx context.Context, _ *container.IPTablesRequest) {
		assert.NoError(t, cleanupCtx.Err())
		_, hasDeadline := cleanupCtx.Deadline()
		assert.True(t, hasDeadline)
	}).Return(nil).Once()

	assert.Error(t, runIPTables(ctx, mockClient, addReq, delReq, &fakeReport{}))
}

func Test_runIPTables_dryRunReturnsWithoutWaiting(t *testing.T) {
	mockClient := container.NewMockClient(t)
	ctr := &container.Container{ContainerName: "c1"}
	addReq := &container.IPTablesRequest{Container: ctr, CmdPrefix: []string{"-I", "INPUT"}, Duration: time.Hour, DryRun: true}
	delReq := &container.IPTablesRequest{Container: ctr, CmdPrefix: []string{"-D", "INPUT"}, Duration: time.Hour, DryRun: true}
	mockClient.EXPECT().IPTablesContainer(mock.Anything, addReq).Return(nil).Once()
	mockClient.EXPECT().StopIPTablesContainer(mock.Anything, delReq).Return(nil).Once()

	start := time.Now()
	require.NoError(t, runIPTables(context.Background(), mockClient, addReq, delReq, &fakeReport{}))
	assert.Less(t, time.Since(start), time.Second, "dry-run must not sleep for --duration")
}
