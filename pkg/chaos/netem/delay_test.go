package netem

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
)

func TestNewDelayCommand(t *testing.T) {
	type args struct {
		names        []string
		pattern      string
		labels       []string
		iface        string
		ipsList      []string
		sportsList   string
		dportsList   string
		durationStr  string
		intervalStr  string
		time         int
		jitter       int
		correlation  float64
		distribution string
		image        string
		pull         bool
		limit        int
		dryRun       bool
	}
	tests := []struct {
		name    string
		args    args
		want    chaos.Command
		wantErr bool
	}{
		{
			name: "create Delay command",
			args: args{
				names:        []string{"n1", "n2"},
				pattern:      "re2:test",
				iface:        "testIface",
				ipsList:      []string{"1.2.3.4", "5.6.7.8"},
				sportsList:   "0,65535",
				dportsList:   "33,512",
				intervalStr:  "1m",
				durationStr:  "30s",
				time:         10,
				jitter:       2,
				correlation:  0.5,
				distribution: delayDistribution[2],
				image:        "test/image",
				limit:        2,
			},
			want: &DelayCommand{
				client:  nil,
				names:   []string{"n1", "n2"},
				pattern: "re2:test",
				iface:   "testIface",
				ips: []*net.IPNet{
					{IP: net.IP{1, 2, 3, 4}, Mask: net.IPMask{255, 255, 255, 255}},
					{IP: net.IP{5, 6, 7, 8}, Mask: net.IPMask{255, 255, 255, 255}},
				},
				sports:       []string{"0", "65535"},
				dports:       []string{"33", "512"},
				duration:     30 * time.Second,
				time:         10,
				jitter:       2,
				correlation:  0.5,
				distribution: delayDistribution[2],
				image:        "test/image",
				limit:        2,
			},
		},
		{
			name: "bad interval value",
			args: args{
				intervalStr: "bad-interval",
			},
			wantErr: true,
		},
		{
			name: "bad duration value",
			args: args{
				intervalStr: "1m",
				durationStr: "bad-duration",
			},
			wantErr: true,
		},
		{
			name: "duration is bigger than interval value",
			args: args{
				intervalStr: "1m",
				durationStr: "2m",
			},
			wantErr: true,
		},
		{
			name: "bad network interface name",
			args: args{
				intervalStr: "1m",
				durationStr: "30s",
				iface:       "bad#interface",
			},
			wantErr: true,
		},
		{
			name: "invalid CIDR IP address",
			args: args{
				intervalStr: "1m",
				durationStr: "30s",
				iface:       "eth0",
				ipsList:     []string{"1.2.3.4/3.4.5.6..."},
			},
			wantErr: true,
		},
		{
			name: "invalid IP address",
			args: args{
				intervalStr: "1m",
				durationStr: "30s",
				iface:       "eth0",
				ipsList:     []string{"1.2.3.4.5.6..."},
			},
			wantErr: true,
		},
		{
			name: "invalid port number: too high",
			args: args{
				intervalStr: "1m",
				durationStr: "30s",
				iface:       "eth0",
				sportsList:  "70000",
			},
			wantErr: true,
		},
		{
			name: "invalid port number: non-numerical",
			args: args{
				intervalStr: "1m",
				durationStr: "30s",
				iface:       "eth0",
				dportsList:  "!abcd$",
			},
			wantErr: true,
		},
		{
			name: "negative delay time",
			args: args{
				intervalStr: "1m",
				durationStr: "30s",
				iface:       "eth0",
				ipsList:     []string{"1.2.3.4"},
				time:        -1,
			},
			wantErr: true,
		},
		{
			name: "negative jitter time",
			args: args{
				intervalStr: "1m",
				durationStr: "30s",
				iface:       "eth0",
				ipsList:     []string{"1.2.3.4"},
				time:        1,
				jitter:      -1,
			},
			wantErr: true,
		},
		{
			name: "too big jitter time",
			args: args{
				intervalStr: "1m",
				durationStr: "30s",
				iface:       "eth0",
				ipsList:     []string{"1.2.3.4"},
				time:        1,
				jitter:      2,
			},
			wantErr: true,
		},
		{
			name: "bad correlation",
			args: args{
				intervalStr: "1m",
				durationStr: "30s",
				iface:       "eth0",
				ipsList:     []string{"1.2.3.4"},
				time:        10,
				jitter:      2,
				correlation: 101.0,
			},
			wantErr: true,
		},
		{
			name: "bad distribution",
			args: args{
				intervalStr:  "1m",
				durationStr:  "30s",
				iface:        "eth0",
				ipsList:      []string{"1.2.3.4"},
				time:         10,
				jitter:       2,
				correlation:  11.0,
				distribution: "bad-distribution",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// invoke
			got, err := NewDelayCommand(nil, tt.args.names, tt.args.pattern, tt.args.labels, tt.args.iface, tt.args.ipsList, tt.args.sportsList, tt.args.dportsList, tt.args.durationStr, tt.args.intervalStr, tt.args.time, tt.args.jitter, tt.args.correlation, tt.args.distribution, tt.args.image, tt.args.pull, tt.args.limit, tt.args.dryRun)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDelayCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewDelayCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDelayCommand_Run(t *testing.T) {
	type wantErrors struct {
		listError  bool
		netemError bool
	}
	type fields struct {
		names        []string
		pattern      string
		iface        string
		ips          []*net.IPNet
		sports       []string
		dports       []string
		duration     time.Duration
		time         int
		jitter       int
		correlation  float64
		distribution string
		image        string
		pull         bool
		limit        int
		dryRun       bool
	}
	type args struct {
		random bool
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		cmd      []string
		expected []*container.Container
		wantErr  bool
		errs     wantErrors
	}{
		{
			name: "delay with CIDR IP",
			fields: fields{
				names:        []string{"c1"},
				iface:        "eth0",
				ips:          []*net.IPNet{{IP: net.IP{10, 10, 10, 10}, Mask: net.IPMask{0, 255, 255, 255}}},
				duration:     10 * time.Microsecond,
				time:         2,
				jitter:       1,
				correlation:  10.0,
				distribution: "normal",
			},
			expected: container.CreateTestContainers(1),
			cmd:      []string{"delay", "2ms", "1ms", "10.00", "distribution", "normal"},
		},
		{
			name: "delay with sport",
			fields: fields{
				names:        []string{"c1"},
				iface:        "eth0",
				sports:       []string{"33"},
				duration:     10 * time.Microsecond,
				time:         2,
				jitter:       1,
				correlation:  10.0,
				distribution: "normal",
			},
			expected: container.CreateTestContainers(1),
			cmd:      []string{"delay", "2ms", "1ms", "10.00", "distribution", "normal"},
		},
		{
			name: "delay with dport",
			fields: fields{
				names:        []string{"c1"},
				iface:        "eth0",
				dports:       []string{"512"},
				duration:     10 * time.Microsecond,
				time:         2,
				jitter:       1,
				correlation:  10.0,
				distribution: "normal",
			},
			expected: container.CreateTestContainers(1),
			cmd:      []string{"delay", "2ms", "1ms", "10.00", "distribution", "normal"},
		},
		{
			name: "delay single container",
			fields: fields{
				names:        []string{"c1"},
				iface:        "eth0",
				ips:          []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				duration:     10 * time.Microsecond,
				time:         2,
				jitter:       1,
				correlation:  10.0,
				distribution: "normal",
			},
			expected: container.CreateTestContainers(1),
			cmd:      []string{"delay", "2ms", "1ms", "10.00", "distribution", "normal"},
		},
		{
			name: "delay multiple container",
			fields: fields{
				names:        []string{"c1", "c2", "c3"},
				iface:        "eth0",
				ips:          []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				duration:     10 * time.Microsecond,
				time:         2,
				jitter:       1,
				correlation:  10.0,
				distribution: "normal",
			},
			expected: container.CreateTestContainers(3),
			cmd:      []string{"delay", "2ms", "1ms", "10.00", "distribution", "normal"},
		},
		{
			name: "delay random container",
			fields: fields{
				names:        []string{"c1", "c2", "c3"},
				iface:        "eth0",
				ips:          []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				duration:     10 * time.Microsecond,
				time:         2,
				jitter:       1,
				correlation:  10.0,
				distribution: "normal",
			},
			args:     args{random: true},
			expected: container.CreateTestContainers(1),
			cmd:      []string{"delay", "2ms", "1ms", "10.00", "distribution", "normal"},
		},
		{
			name: "no container found",
			fields: fields{
				names: []string{"c1"},
			},
		},
		{
			name: "error listing containers",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			wantErr: true,
			errs:    wantErrors{listError: true},
		},
		{
			name: "error delaying container",
			fields: fields{
				names:        []string{"c1", "c2", "c3"},
				iface:        "eth0",
				ips:          []*net.IPNet{{IP: net.IP{10, 10, 10, 10}}},
				duration:     10 * time.Microsecond,
				time:         2,
				jitter:       1,
				correlation:  10.0,
				distribution: "normal",
			},
			expected: container.CreateTestContainers(3),
			cmd:      []string{"delay", "2ms", "1ms", "10.00", "distribution", "normal"},
			wantErr:  true,
			errs:     wantErrors{netemError: true},
		},
	}
	for _, tt := range tests {
		mockClient := new(container.MockClient)
		t.Run(tt.name, func(t *testing.T) {
			n := &DelayCommand{
				client:       mockClient,
				names:        tt.fields.names,
				pattern:      tt.fields.pattern,
				iface:        tt.fields.iface,
				ips:          tt.fields.ips,
				sports:       tt.fields.sports,
				dports:       tt.fields.dports,
				duration:     tt.fields.duration,
				time:         tt.fields.time,
				jitter:       tt.fields.jitter,
				correlation:  tt.fields.correlation,
				distribution: tt.fields.distribution,
				image:        tt.fields.image,
				limit:        tt.fields.limit,
				dryRun:       tt.fields.dryRun,
			}
			// mock calls
			call := mockClient.On("ListContainers", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts"))
			if tt.errs.listError {
				call.Return(tt.expected, errors.New("ERROR"))
				goto Invoke
			} else {
				call.Return(tt.expected, nil)
				if tt.expected == nil {
					goto Invoke
				}
			}
			if tt.args.random {
				mockClient.On("NetemContainer", mock.AnythingOfType("*context.cancelCtx"), mock.AnythingOfType("*container.Container"), tt.fields.iface, tt.cmd, tt.fields.ips, tt.fields.sports, tt.fields.dports, tt.fields.duration, tt.fields.image, tt.fields.pull, tt.fields.dryRun).Return(nil)
				mockClient.On("StopNetemContainer", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*container.Container"), tt.fields.iface, tt.fields.ips, tt.fields.sports, tt.fields.dports, tt.fields.image, tt.fields.pull, tt.fields.dryRun).Return(nil)
			} else {
				for i := range tt.expected {
					if tt.fields.limit == 0 || i < tt.fields.limit {
						call = mockClient.On("NetemContainer", mock.AnythingOfType("*context.cancelCtx"), mock.AnythingOfType("*container.Container"), tt.fields.iface, tt.cmd, tt.fields.ips, tt.fields.sports, tt.fields.dports, tt.fields.duration, tt.fields.image, tt.fields.pull, tt.fields.dryRun)
						if tt.errs.netemError {
							call.Return(errors.New("ERROR"))
							goto Invoke
						} else {
							call.Return(nil)
						}
						mockClient.On("StopNetemContainer", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*container.Container"), tt.fields.iface, tt.fields.ips, tt.fields.sports, tt.fields.dports, tt.fields.image, tt.fields.pull, tt.fields.dryRun).Return(nil)
					}
				}
			}
		Invoke:
			if err := n.Run(context.TODO(), tt.args.random); (err != nil) != tt.wantErr {
				t.Errorf("DelayCommand.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			// asset mock
			mockClient.AssertExpectations(t)
		})
	}
}
