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
	"SIGIOT":    syscall.Signal(0x6), //nolint:gomnd SIGIOT (use number since this signal is not defined for Windows)
	"SIGBUS":    syscall.SIGBUS,
	"SIGFPE":    syscall.SIGFPE,
	"SIGKILL":   syscall.SIGKILL,
	"SIGUSR1":   syscall.Signal(0x1e), //nolint:gomnd SIGUSR1 (use number since this signal is not defined for Windows)
	"SIGSEGV":   syscall.SIGSEGV,
	"SIGUSR2":   syscall.Signal(0x1f), //nolint:gomnd SIGUSR2 (use number since this signal is not defined for Windows)
	"SIGPIPE":   syscall.SIGPIPE,
	"SIGALRM":   syscall.SIGALRM,
	"SIGTERM":   syscall.SIGTERM,
	"SIGCHLD":   syscall.Signal(0x14), //nolint:gomnd SIGCHLD (use number since this signal is not defined for Windows)
	"SIGCONT":   syscall.Signal(0x13), //nolint:gomnd SIGCONT (use number since this signal is not defined for Windows)
	"SIGSTOP":   syscall.Signal(0x11), //nolint:gomnd SIGSTOP (use number since this signal is not defined for Windows)
	"SIGTSTP":   syscall.Signal(0x12), //nolint:gomnd SIGTSTP (use number since this signal is not defined for Windows)
	"SIGTTIN":   syscall.Signal(0x15), //nolint:gomnd SIGTTIN (use number since this signal is not defined for Windows)
	"SIGTTOU":   syscall.Signal(0x16), //nolint:gomnd SIGTTOU (use number since this signal is not defined for Windows)
	"SIGURG":    syscall.Signal(0x10), //nolint:gomnd SIGURG (use number since this signal is not defined for Windows)
	"SIGXCPU":   syscall.Signal(0x18), //nolint:gomnd SIGXCPU (use number since this signal is not defined for Windows)
	"SIGXFSZ":   syscall.Signal(0x19), //nolint:gomnd SIGXFSZ (use number since this signal is not defined for Windows)
	"SIGVTALRM": syscall.Signal(0x1a), //nolint:gomnd SIGVTALRM (use number since this signal is not defined for Windows)
	"SIGPROF":   syscall.Signal(0x1b), //nolint:gomnd SIGPROF (use number since this signal is not defined for Windows)
	"SIGWINCH":  syscall.Signal(0x1c), //nolint:gomnd SIGWINCH (use number since this signal is not defined for Windows)
	"SIGIO":     syscall.Signal(0x17), //nolint:gomnd SIGIO (use number since this signal is not defined for Windows)
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
