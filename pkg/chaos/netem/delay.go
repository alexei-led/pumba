package netem

import (
	"context"
	"net"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
)

// NetemDelayCommand `netem delay` command
type NetemDelayCommand struct {
	NetInterface string
	IPs          []net.IP
	Duration     time.Duration
	Time         int
	Jitter       int
	Correlation  float64
	Distribution string
	Image        string
}

// NewStopCommand create new Stop Command instance
func NewNetemDelayCommand(client container.Client, names []string, pattern string) (chaos.Command, error) {
	return nil, nil
}

// Run netem delay command
func (s *NetemDelayCommand) Run(ctx context.Context, random bool) error {
	return nil
}
