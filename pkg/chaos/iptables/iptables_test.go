package iptables

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
)

func Test_runIPTables(t *testing.T) {
	type errs struct {
		startErr bool
		stopErr  bool
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
			name: "iptables error in IPTablesContainer",
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
			errs:    errs{startErr: true},
			wantErr: true,
		},
		{
			name: "iptables error in StopIPTablesContainer",
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
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := container.NewMockClient(t)
			ctx, cancel := context.WithCancel(context.TODO())

			startErr := error(nil)
			if tt.errs.startErr {
				startErr = errors.New("test error")
			}
			mockClient.EXPECT().IPTablesContainer(ctx, tt.args.container, tt.args.addCmdPrefix, tt.args.cmdSuffix, tt.args.srcIPs, tt.args.dstIPs, tt.args.sports, tt.args.dports, tt.args.duration, tt.args.image, tt.args.pull, tt.args.dryRun).Return(startErr)

			if !tt.errs.startErr {
				stopErr := error(nil)
				if tt.errs.stopErr {
					stopErr = errors.New("test error")
				}
				mockClient.EXPECT().StopIPTablesContainer(mock.Anything, tt.args.container, tt.args.delCmdPrefix, tt.args.cmdSuffix, tt.args.srcIPs, tt.args.dstIPs, tt.args.sports, tt.args.dports, tt.args.image, tt.args.pull, tt.args.dryRun).Return(stopErr)
			}

			if err := runIPTables(ctx, mockClient, tt.args.container, tt.args.addCmdPrefix, tt.args.delCmdPrefix, tt.args.cmdSuffix, tt.args.srcIPs, tt.args.dstIPs, tt.args.sports, tt.args.dports, tt.args.duration, tt.args.image, tt.args.pull, tt.args.dryRun); (err != nil) != tt.wantErr {
				t.Errorf("runIPTables() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.abort {
				t.Log("cancel iptables")
				cancel()
			} else {
				t.Log("timeout iptables")
				defer cancel()
			}
		})
	}
}
