package docker

import (
	"context"
	"syscall"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	// DefaultKillSignal default kill signal
	DefaultKillSignal = "SIGKILL"
)

// valid Linux signal table
// http://www.comptechdoc.org/os/linux/programming/linux_pgsignals.html
var linuxSignals = map[string]syscall.Signal{
	"SIGHUP":    syscall.SIGHUP,
	"SIGINT":    syscall.SIGINT,
	"SIGQUIT":   syscall.SIGQUIT,
	"SIGILL":    syscall.SIGILL,
	"SIGTRAP":   syscall.SIGTRAP,
	"SIGIOT":    syscall.SIGIOT,
	"SIGBUS":    syscall.SIGBUS,
	"SIGFPE":    syscall.SIGFPE,
	"SIGKILL":   syscall.SIGKILL,
	"SIGUSR1":   syscall.SIGUSR1,
	"SIGSEGV":   syscall.SIGSEGV,
	"SIGUSR2":   syscall.SIGUSR2,
	"SIGPIPE":   syscall.SIGPIPE,
	"SIGALRM":   syscall.SIGALRM,
	"SIGTERM":   syscall.SIGTERM,
	"SIGSTKFLT": 16, //nolint:gomnd
	"SIGCHLD":   syscall.SIGCHLD,
	"SIGCONT":   syscall.SIGCONT,
	"SIGSTOP":   syscall.SIGSTOP,
	"SIGTSTP":   syscall.SIGTSTP,
	"SIGTTIN":   syscall.SIGTTIN,
	"SIGTTOU":   syscall.SIGTTOU,
	"SIGURG":    syscall.SIGURG,
	"SIGXCPU":   syscall.SIGXCPU,
	"SIGXFSZ":   syscall.SIGXFSZ,
	"SIGVTALRM": syscall.SIGVTALRM,
	"SIGPROF":   syscall.SIGPROF,
	"SIGWINCH":  syscall.SIGWINCH,
	"SIGIO":     syscall.SIGIO,
	"SIGPWR":    30, //nolint:gomnd
}

// `docker kill` command
type killCommand struct {
	client  container.Client
	names   []string
	pattern string
	labels  []string
	signal  string
	limit   int
	dryRun  bool
}

// NewKillCommand create new Kill Command instance
func NewKillCommand(client container.Client, params *chaos.GlobalParams, signal string, limit int) (chaos.Command, error) {
	kill := &killCommand{
		client:  client,
		names:   params.Names,
		pattern: params.Pattern,
		labels:  params.Labels,
		signal:  signal,
		limit:   limit,
		dryRun:  params.DryRun,
	}
	if kill.signal == "" {
		kill.signal = DefaultKillSignal
	}
	if _, ok := linuxSignals[kill.signal]; !ok {
		return nil, errors.Errorf("undefined Linux signal: %s", signal)
	}
	return kill, nil
}

// Run kill command
func (k *killCommand) Run(ctx context.Context, random bool) error {
	log.Debug("killing all matching containers")
	log.WithFields(log.Fields{
		"names":   k.names,
		"pattern": k.pattern,
		"labels":  k.labels,
		"limit":   k.limit,
		"random":  random,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, k.client, k.names, k.pattern, k.labels, k.limit)
	if err != nil {
		return errors.Wrap(err, "error listing containers")
	}
	if len(containers) == 0 {
		log.Warning("no containers to kill")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		if c := container.RandomContainer(containers); c != nil {
			containers = []*container.Container{c}
		}
	}

	for _, container := range containers {
		log.WithFields(log.Fields{
			"container": container,
			"signal":    k.signal,
		}).Debug("killing container")
		c := container
		err = k.client.KillContainer(ctx, c, k.signal, k.dryRun)
		if err != nil {
			return errors.Wrap(err, "failed to kill container")
		}
	}
	return nil
}
