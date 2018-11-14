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
		container    container.Container
		netInterface string
		cmd          []string
		ips          []net.IP
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
				container: *container.NewContainer(
					container.ContainerDetailsResponse(container.AsMap("Name", "c1")),
					container.ImageDetailsResponse(container.AsMap()),
				),
				netInterface: "testIface",
				cmd:          []string{"test", "--test"},
				ips:          []net.IP{net.ParseIP("10.10.10.10")},
				duration:     time.Microsecond * 10,
				tcimage:      "test/image",
			},
		},
		{
			name: "netem with abort",
			args: args{
				container: *container.NewContainer(
					container.ContainerDetailsResponse(container.AsMap("Name", "c1")),
					container.ImageDetailsResponse(container.AsMap()),
				),
				netInterface: "testIface",
				cmd:          []string{"test", "--test"},
				ips:          []net.IP{net.ParseIP("10.10.10.10")},
				duration:     time.Microsecond * 10,
				tcimage:      "test/image",
			},
			abort: true,
		},
		{
			name: "netem error in NetemContainer",
			args: args{
				container: *container.NewContainer(
					container.ContainerDetailsResponse(container.AsMap("Name", "c1")),
					container.ImageDetailsResponse(container.AsMap()),
				),
				netInterface: "testIface",
				cmd:          []string{"test", "--test"},
				ips:          []net.IP{net.ParseIP("10.10.10.10")},
				duration:     time.Microsecond * 10,
				tcimage:      "test/image",
			},
			errs:    errs{startErr: true},
			wantErr: true,
		},
		{
			name: "netem error in StopNetemContainer",
			args: args{
				container: *container.NewContainer(
					container.ContainerDetailsResponse(container.AsMap("Name", "c1")),
					container.ImageDetailsResponse(container.AsMap()),
				),
				netInterface: "testIface",
				cmd:          []string{"test", "--test"},
				ips:          []net.IP{net.ParseIP("10.10.10.10")},
				duration:     time.Microsecond * 10,
				tcimage:      "test/image",
			},
			errs:    errs{stopErr: true},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create client mock
			mockClient := &container.MockClient{}
			// create timeout context
			ctx, cancel := context.WithCancel(context.TODO())
			// set NetemContainer mock call
			call := mockClient.On("NetemContainer", ctx, tt.args.container, tt.args.netInterface, tt.args.cmd, tt.args.ips, tt.args.duration, tt.args.tcimage, tt.args.pull, tt.args.dryRun)
			if tt.errs.startErr {
				call.Return(errors.New("test error"))
				goto Invoke
			} else {
				call.Return(nil)
			}
			// set StopNetemContainer mock call
			call = mockClient.On("StopNetemContainer", mock.AnythingOfType("*context.emptyCtx"), tt.args.container, tt.args.netInterface, tt.args.ips, tt.args.tcimage, tt.args.pull, tt.args.dryRun)
			if tt.errs.stopErr {
				call.Return(errors.New("test error"))
			} else {
				call.Return(nil)
			}
			// invoke
		Invoke:
			if err := runNetem(ctx, mockClient, tt.args.container, tt.args.netInterface, tt.args.cmd, tt.args.ips, tt.args.duration, tt.args.tcimage, tt.args.pull, tt.args.dryRun); (err != nil) != tt.wantErr {
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
