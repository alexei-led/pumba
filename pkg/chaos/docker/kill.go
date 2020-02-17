package docker

import (
	"context"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	// DefaultKillSignal default kill signal
	DefaultKillSignal = "SIGKILL"
)

// LinuxSignals valid Linux signal table
// http://www.comptechdoc.org/os/linux/programming/linux_pgsignals.html
var LinuxSignals = map[string]int{
	"SIGHUP":    1,
	"SIGINT":    2,
	"SIGQUIT":   3,
	"SIGILL":    4,
	"SIGTRAP":   5,
	"SIGIOT":    6,
	"SIGBUS":    7,
	"SIGFPE":    8,
	"SIGKILL":   9,
	"SIGUSR1":   10,
	"SIGSEGV":   11,
	"SIGUSR2":   12,
	"SIGPIPE":   13,
	"SIGALRM":   14,
	"SIGTERM":   15,
	"SIGSTKFLT": 16,
	"SIGCHLD":   17,
	"SIGCONT":   18,
	"SIGSTOP":   19,
	"SIGTSTP":   20,
	"SIGTTIN":   21,
	"SIGTTOU":   22,
	"SIGURG":    23,
	"SIGXCPU":   24,
	"SIGXFSZ":   25,
	"SIGVTALRM": 26,
	"SIGPROF":   27,
	"SIGWINCH":  28,
	"SIGIO":     29,
	"SIGPWR":    30,
}

// KillCommand `docker kill` command
type KillCommand struct {
	client  container.Client
	names   []string
	pattern string
	labels  []string
	signal  string
	limit   int
	dryRun  bool
}

// NewKillCommand create new Kill Command instance
func NewKillCommand(client container.Client, names []string, pattern string, labels []string, signal string, limit int, dryRun bool) (chaos.Command, error) {
	kill := &KillCommand{client, names, pattern, labels, signal, limit, dryRun}
	if kill.signal == "" {
		kill.signal = DefaultKillSignal
	}
	if _, ok := LinuxSignals[kill.signal]; !ok {
		return nil, errors.Errorf("undefined Linux signal: %s", signal)
	}
	return kill, nil
}

// Run kill command
func (k *KillCommand) Run(ctx context.Context, random bool) error {
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
		return err
	}
	if len(containers) == 0 {
		log.Warning("no containers to kill")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		if c := container.RandomContainer(containers); c != nil {
			containers = []container.Container{*c}
		}
	}

	for _, container := range containers {
		log.WithFields(log.Fields{
			"container": container,
			"signal":    k.signal,
		}).Debug("killing container")
		err := k.client.KillContainer(ctx, container, k.signal, k.dryRun)
		if err != nil {
			return errors.Wrap(err, "failed to kill container")
		}
	}
	return nil
}
