package iptables

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
)

func Test_runIPTables(t *testing.T) {
	type errs struct {
		startErr bool
		stopErr  bool
	}
	type args struct {
		container     *container.Container
		cmdPrefix     []string
		cmdSuffix     []string
		dstIPs        []*net.IPNet
		srcIPs        []*net.IPNet
		sports        []string
		dports        []string
		duration      time.Duration
		iptablesImage string
		pull          bool
		dryRun        bool
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
					ContainerInfo: container.DetailsResponse(container.AsMap("Name", "c1")),
					ImageInfo:     container.ImageDetailsResponse(container.AsMap()),
				},
				cmdPrefix:     []string{"test", "--test"},
				cmdSuffix:     []string{"test", "--test"},
				dstIPs:        []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				srcIPs:        []*net.IPNet{{IP: net.IP{10, 10, 10, 1}}},
				sports:        []string{"44"},
				dports:        []string{"662"},
				duration:      time.Microsecond * 10,
				iptablesImage: "test/image",
			},
		},
		{
			name: "iptables with CIDR IP",
			args: args{
				container: &container.Container{
					ContainerInfo: container.DetailsResponse(container.AsMap("Name", "c1")),
					ImageInfo:     container.ImageDetailsResponse(container.AsMap()),
				},
				cmdPrefix:     []string{"test", "--test"},
				cmdSuffix:     []string{"test", "--test"},
				dstIPs:        []*net.IPNet{{IP: net.IP{10, 10, 10, 10}, Mask: net.IPMask{0, 0, 255, 255}}},
				srcIPs:        []*net.IPNet{{IP: net.IP{10, 10, 10, 1}, Mask: net.IPMask{0, 0, 255, 255}}},
				duration:      time.Microsecond * 10,
				iptablesImage: "test/image",
			},
		},
		{
			name: "iptables with abort",
			args: args{
				container: &container.Container{
					ContainerInfo: container.DetailsResponse(container.AsMap("Name", "c1")),
					ImageInfo:     container.ImageDetailsResponse(container.AsMap()),
				},
				cmdPrefix:     []string{"test", "--test"},
				cmdSuffix:     []string{"test", "--test"},
				dstIPs:        []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				duration:      time.Microsecond * 10,
				iptablesImage: "test/image",
			},
			abort: true,
		},
		{
			name: "iptables error in IPTablesContainer",
			args: args{
				container: &container.Container{
					ContainerInfo: container.DetailsResponse(container.AsMap("Name", "c1")),
					ImageInfo:     container.ImageDetailsResponse(container.AsMap()),
				},
				cmdPrefix:     []string{"test", "--test"},
				cmdSuffix:     []string{"test", "--test"},
				dstIPs:        []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				duration:      time.Microsecond * 10,
				iptablesImage: "test/image",
			},
			errs:    errs{startErr: true},
			wantErr: true,
		},
		{
			name: "iptables error in StopIPTablesContainer",
			args: args{
				container: &container.Container{
					ContainerInfo: container.DetailsResponse(container.AsMap("Name", "c1")),
					ImageInfo:     container.ImageDetailsResponse(container.AsMap()),
				},
				cmdPrefix:     []string{"test", "--test"},
				cmdSuffix:     []string{"test", "--test"},
				dstIPs:        []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				duration:      time.Microsecond * 10,
				iptablesImage: "test/image",
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
			// set IPTablesContainer mock call
			call := mockClient.On("IPTablesContainer", ctx, tt.args.container, tt.args.cmdPrefix, tt.args.cmdSuffix, tt.args.srcIPs, tt.args.dstIPs, tt.args.sports, tt.args.dports, tt.args.duration, tt.args.iptablesImage, tt.args.pull, tt.args.dryRun)
			if tt.errs.startErr {
				call.Return(errors.New("test error"))
				goto Invoke
			} else {
				call.Return(nil)
			}
			// set StopContainer mock call
			call = mockClient.On("StopIPTablesContainer", context.Background(), tt.args.container, tt.args.cmdPrefix, tt.args.cmdSuffix, tt.args.srcIPs, tt.args.dstIPs, tt.args.sports, tt.args.dports, tt.args.iptablesImage, tt.args.pull, tt.args.dryRun)
			if tt.errs.stopErr {
				call.Return(errors.New("test error"))
			} else {
				call.Return(nil)
			}
			// invoke
		Invoke:
			if err := runIPTables(ctx, mockClient, tt.args.container, tt.args.cmdPrefix, tt.args.cmdPrefix, tt.args.cmdSuffix, tt.args.srcIPs, tt.args.dstIPs, tt.args.sports, tt.args.dports, tt.args.duration, tt.args.iptablesImage, tt.args.pull, tt.args.dryRun); (err != nil) != tt.wantErr {
				t.Errorf("runIPTables() error = %v, wantErr %v", err, tt.wantErr)
			}
			// abort
			if tt.abort {
				t.Log("cancel iptables")
				cancel()
			} else {
				t.Log("timeout iptables")
				defer cancel()
			}
			// asset mock
			mockClient.AssertExpectations(t)
		})
	}
}
