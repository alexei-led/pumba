package netem

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_runNetem(t *testing.T) {
	type errs struct {
		startErr bool
		stopErr  bool
	}
	type args struct {
		container    *container.Container
		netInterface string
		cmd          []string
		ips          []*net.IPNet
		sports       []string
		dports       []string
		duration     time.Duration
		tcimage      string
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
			name: "netem with duration",
			args: args{
				container: &container.Container{
					ContainerName: "c1",
					Labels:        map[string]string{},
					Networks:      map[string]container.NetworkLink{},
				},
				netInterface: "testIface",
				cmd:          []string{"test", "--test"},
				ips:          []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				sports:       []string{"44"},
				dports:       []string{"662"},
				duration:     time.Microsecond * 10,
				tcimage:      "test/image",
			},
		},
		{
			name: "netem with CIDR IP",
			args: args{
				container: &container.Container{
					ContainerName: "c1",
					Labels:        map[string]string{},
					Networks:      map[string]container.NetworkLink{},
				},
				netInterface: "testIface",
				cmd:          []string{"test", "--test"},
				ips:          []*net.IPNet{{IP: net.IP{10, 10, 10, 10}, Mask: net.IPMask{0, 0, 255, 255}}},
				duration:     time.Microsecond * 10,
				tcimage:      "test/image",
			},
		},
		{
			name: "netem with abort",
			args: args{
				container: &container.Container{
					ContainerName: "c1",
					Labels:        map[string]string{},
					Networks:      map[string]container.NetworkLink{},
				},
				netInterface: "testIface",
				cmd:          []string{"test", "--test"},
				ips:          []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				duration:     time.Microsecond * 10,
				tcimage:      "test/image",
			},
			abort: true,
		},
		{
			name: "netem error in NetemContainer",
			args: args{
				container: &container.Container{
					ContainerName: "c1",
					Labels:        map[string]string{},
					Networks:      map[string]container.NetworkLink{},
				},
				netInterface: "testIface",
				cmd:          []string{"test", "--test"},
				ips:          []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				duration:     time.Microsecond * 10,
				tcimage:      "test/image",
			},
			errs:    errs{startErr: true},
			wantErr: true,
		},
		{
			name: "netem timeout returns StopNetemContainer failure",
			args: args{
				container: &container.Container{
					ContainerName: "c1",
					Labels:        map[string]string{},
					Networks:      map[string]container.NetworkLink{},
				},
				netInterface: "testIface",
				cmd:          []string{"test", "--test"},
				ips:          []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				duration:     time.Microsecond * 10,
				tcimage:      "test/image",
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

			startErr := error(nil)
			if tt.errs.startErr {
				startErr = errors.New("test error")
			}
			req := &container.NetemRequest{
				Container: tt.args.container,
				Interface: tt.args.netInterface,
				Command:   tt.args.cmd,
				IPs:       tt.args.ips,
				SPorts:    tt.args.sports,
				DPorts:    tt.args.dports,
				Duration:  tt.args.duration,
				Sidecar:   container.SidecarSpec{Image: tt.args.tcimage, Pull: tt.args.pull},
				DryRun:    tt.args.dryRun,
			}
			mockClient.EXPECT().NetemContainer(ctx, req).Return(startErr)

			if !tt.errs.startErr {
				stopErr := error(nil)
				if tt.errs.stopErr {
					stopErr = errors.New("test error")
				}
				mockClient.EXPECT().StopNetemContainer(mock.Anything, req).Return(stopErr)
			}

			// abort case: cancel ctx before runNetem so the ctx.Done() branch
			// is exercised; otherwise the stopCtx timeout branch wins.
			if tt.abort {
				cancel()
			}

			if err := runNetem(ctx, mockClient, req); (err != nil) != tt.wantErr {
				t.Errorf("runNetem() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunNetemTimeoutReturnsCleanupError(t *testing.T) {
	req := &container.NetemRequest{
		Container: &container.Container{ContainerID: "c1"},
		Interface: "eth0",
		Duration:  0,
	}
	stopErr := errors.New("cleanup failed")
	client := container.NewMockClient(t)
	client.EXPECT().NetemContainer(mock.Anything, req).Return(nil)
	client.EXPECT().StopNetemContainer(mock.Anything, req).
		Run(func(cleanupCtx context.Context, _ *container.NetemRequest) {
			require.NoError(t, cleanupCtx.Err(), "cleanup timeout must be independent of the run timeout")
		}).
		Return(stopErr)

	err := runNetem(context.Background(), client, req)
	require.ErrorIs(t, err, stopErr)
}

func TestRunNetemCancellationReturnsCleanupErrorWithLiveContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := &container.NetemRequest{
		Container: &container.Container{ContainerID: "c1"},
		Interface: "eth0",
		Duration:  time.Hour,
	}
	stopErr := errors.New("cleanup failed")
	client := container.NewMockClient(t)
	client.EXPECT().NetemContainer(ctx, req).Return(nil)
	client.EXPECT().StopNetemContainer(mock.Anything, req).
		Run(func(cleanupCtx context.Context, _ *container.NetemRequest) {
			require.NoError(t, cleanupCtx.Err(), "cleanup must outlive parent cancellation")
			deadline, ok := cleanupCtx.Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(cleanupTimeout), deadline, time.Second)
		}).
		Return(stopErr)

	err := runNetem(ctx, client, req)
	require.ErrorIs(t, err, stopErr)
}
