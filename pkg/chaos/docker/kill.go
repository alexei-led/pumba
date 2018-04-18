package docker

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
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
	signal  string
	limit   int
	dryRun  bool
}

// NewKillCommand create new Kill Command instance
func NewKillCommand(client container.Client, names []string, pattern string, signal string, limit int, dryRun bool) (chaos.Command, error) {
	kill := &KillCommand{client, names, pattern, signal, limit, dryRun}
	if kill.signal == "" {
		kill.signal = DefaultKillSignal
	}
	if _, ok := LinuxSignals[kill.signal]; !ok {
		err := fmt.Errorf("undefined Linux signal: %s", signal)
		log.WithError(err).Error("bad value for Linux signal")
		return nil, err
	}
	return kill, nil
}

// Run kill command
func (k *KillCommand) Run(ctx context.Context, random bool) error {
	log.Debug("killing all matching containers")
	log.WithFields(log.Fields{
		"names":   k.names,
		"pattern": k.pattern,
		"limit":   k.limit,
	}).Debug("listing matching containers")
	containers, err := listNContainers(ctx, k.client, k.names, k.pattern, k.limit)
	if err != nil {
		log.WithError(err).Error("failed to list containers")
		return err
	}
	if len(containers) == 0 {
		log.Warning("no containers to kill")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		log.Debug("selecting single random container")
		if c := randomContainer(containers); c != nil {
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
			log.WithError(err).Error("failed to kill container")
			return err
		}
	}
	return nil
}
