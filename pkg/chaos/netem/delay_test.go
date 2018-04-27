package netem

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
)

func TestNewDelayCommand(t *testing.T) {
	type args struct {
		names        []string
		pattern      string
		iface        string
		ipsList      []string
		durationStr  string
		intervalStr  string
		time         int
		jitter       int
		correlation  float64
		distribution string
		image        string
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
				client:       nil,
				names:        []string{"n1", "n2"},
				pattern:      "re2:test",
				iface:        "testIface",
				ips:          []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("5.6.7.8")},
				duration:     30 * time.Second,
				time:         10,
				jitter:       2,
				correlation:  0.5,
				distribution: delayDistribution[2],
				image:        "test/image",
				limit:        2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// invoke
			got, err := NewDelayCommand(nil, tt.args.names, tt.args.pattern, tt.args.iface, tt.args.ipsList, tt.args.durationStr, tt.args.intervalStr, tt.args.time, tt.args.jitter, tt.args.correlation, tt.args.distribution, tt.args.image, tt.args.limit, tt.args.dryRun)
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
