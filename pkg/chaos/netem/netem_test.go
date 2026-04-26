package netem

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
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
			name: "netem warning on StopNetemContainer failure",
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
			wantErr: false,
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
