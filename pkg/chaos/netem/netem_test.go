package netem

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
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
			// create client mock
			mockClient := &container.MockClient{}
			// create timeout context
			ctx, cancel := context.WithCancel(context.TODO())
			// set NetemContainer mock call
			call := mockClient.On("NetemContainer", ctx, tt.args.container, tt.args.netInterface, tt.args.cmd, tt.args.ips, tt.args.sports, tt.args.dports, tt.args.duration, tt.args.tcimage, tt.args.pull, tt.args.dryRun)
			if tt.errs.startErr {
				call.Return(errors.New("test error"))
				goto Invoke
			} else {
				call.Return(nil)
			}
			// set StopNetemContainer mock call
			call = mockClient.On("StopNetemContainer", context.Background(), tt.args.container, tt.args.netInterface, tt.args.ips, tt.args.sports, tt.args.dports, tt.args.tcimage, tt.args.pull, tt.args.dryRun)
			if tt.errs.stopErr {
				call.Return(errors.New("test error"))
			} else {
				call.Return(nil)
			}
			// invoke
		Invoke:
			if err := runNetem(ctx, mockClient, tt.args.container, tt.args.netInterface, tt.args.cmd, tt.args.ips, tt.args.sports, tt.args.dports, tt.args.duration, tt.args.tcimage, tt.args.pull, tt.args.dryRun); (err != nil) != tt.wantErr {
				t.Errorf("runNetem() error = %v, wantErr %v", err, tt.wantErr)
			}
			// abort
			if tt.abort {
				t.Log("cancel netem")
				cancel()
			} else {
				t.Log("timeout netem")
				defer cancel()
			}
			// asset mock
			mockClient.AssertExpectations(t)
		})
	}
}
