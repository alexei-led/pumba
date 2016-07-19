package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gaia-adm/pumba/action"
	"github.com/gaia-adm/pumba/container"

	"github.com/urfave/cli"

	log "github.com/Sirupsen/logrus"
	"github.com/johntdyer/slackrus"
)

var (
	wg               sync.WaitGroup
	client           container.Client
	interval         time.Duration
	containerNames   []string
	containerPattern string
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

const (
	release         = "v0.2.0"
	defaultNetemCmd = "delay 1000ms"
	defaultSignal   = "SIGKILL"
	re2prefix       = "re2:"
)

type commandKill struct {
	signal string
}

type commandPause struct {
	duration time.Duration
}

type commandNetem struct {
	duration time.Duration
}

type commandStop struct {
	waitTime int
}

type commandRemove struct {
	force   bool
	link    string
	volumes string
}

func init() {
	log.SetLevel(log.InfoLevel)
	log.SetFormatter(&log.TextFormatter{})
}

func main() {
	rootCertPath := "/etc/ssl/docker"

	if os.Getenv("DOCKER_CERT_PATH") != "" {
		rootCertPath = os.Getenv("DOCKER_CERT_PATH")
	}

	app := cli.NewApp()
	app.Name = "Pumba"
	app.Version = release
	app.Usage = "Pumba is a resilience testing tool, that helps applications tolerate random Docker container failures: process, network and performance."
	app.ArgsUsage = "containers (name, list of names, RE2 regex)"
	app.Before = before
	app.Commands = []cli.Command{
		{
			Name: "kill",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "signal, s",
					Usage: "Specify termination signal that will be sent by Pumba to process running within target container.",
					Value: defaultSignal,
				},
			},
			Usage:       "kill specified containers",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "Pumba will send SIGKILL signal (by default) to the main process inside target container(s), or any signal specified with option '--signal'.",
			Action:      kill,
			Before:      beforeCommand,
		},
		{
			Name: "netem",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "duration, d",
					Usage: "Specify network emulation duration. It should be smaller than recurrent interval. The network emulation duration is a decimal number with optional unit suffix, such as '500ms', '20s' or '30m'. Valid time units are: 'ms', 's', 'm', 'h'.",
				},
			},
			Usage:       "Perform network emulation action on target containers",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "Pumba will modify network interface settings (run 'netem'), to emulate different problems for target specified containers for specified '--duration'.",
			Before:      beforeCommand,
			Subcommands: []cli.Command{
				{
					Name:   "delay",
					Action: netemDelay,
				},
				{
					Name: "loss",
				},
				{
					Name: "duplicate",
				},
				{
					Name: "corrupt",
				},
			},
		},
		{
			Name: "pause",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "duration, d",
					Usage: "Specify pause duration. It should be smaller than recurrent interval. The pause duration is a decimal number with optional unit suffix, such as '500ms', '20s' or '30m'. Valid time units are: 'ms', 's', 'm', 'h'.",
				},
			},
			Usage:       "pause all processes within specified containers",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "Pumba will pause all running processes within specified containers for specified '--duration'.",
			Action:      pause,
			Before:      beforeCommand,
		},
		{
			Name: "stop",
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "time, t",
					Usage: "Seconds to wait for stop before killing it (default 10)",
					Value: 10,
				},
			},
			Usage:       "stop specified containers",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "Pumba will try to stop the main process inside the specfied container(s), sending  SIGTERM, and then SIGKILL after grace period.",
			Action:      stop,
			Before:      beforeCommand,
		},
		{
			Name: "rm",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "force, f",
					Usage: "Force the removal of a running container (uses SIGKILL).",
				},
				cli.StringFlag{
					Name:  "link, l",
					Usage: "Remove the specified link.",
				},
				cli.StringFlag{
					Name:  "volumes, v",
					Usage: "Remove the volumes associated with the container.",
				},
			},
			Usage:       "remove specified containers",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "Pumba will try remove specfied container(s)",
			Action:      remove,
			Before:      beforeCommand,
		},
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host, H",
			Usage:  "daemon socket to connect to",
			Value:  "unix:///var/run/docker.sock",
			EnvVar: "DOCKER_HOST",
		},
		cli.BoolFlag{
			Name:  "tls",
			Usage: "use TLS; implied by --tlsverify",
		},
		cli.BoolFlag{
			Name:   "tlsverify",
			Usage:  "use TLS and verify the remote",
			EnvVar: "DOCKER_TLS_VERIFY",
		},
		cli.StringFlag{
			Name:  "tlscacert",
			Usage: "trust certs signed only by this CA",
			Value: fmt.Sprintf("%s/ca.pem", rootCertPath),
		},
		cli.StringFlag{
			Name:  "tlscert",
			Usage: "client certificate for TLS authentication",
			Value: fmt.Sprintf("%s/cert.pem", rootCertPath),
		},
		cli.StringFlag{
			Name:  "tlskey",
			Usage: "client key for TLS authentication",
			Value: fmt.Sprintf("%s/key.pem", rootCertPath),
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug mode with verbose logging",
		},
		cli.BoolFlag{
			Name:  "json",
			Usage: "produce log in JSON format: Logstash and Splunk friendly"},
		cli.StringFlag{
			Name:  "slackhook",
			Usage: "Slack web hook url. Send Pumba log events to Slack",
		},
		cli.StringFlag{
			Name:  "slackchannel",
			Usage: "Slack channel for Pumba log events",
			Value: "#pumba",
		},
		cli.StringFlag{
			Name:  "interval, i",
			Usage: "Specify recurrent interval for kill command. The interval is a decimal number with optional unit suffix, such as '500ms', '20s' or '30m'. Valid time units are: 'ms', 's', 'm', 'h'.",
		},
		cli.BoolFlag{
			Name:        "random, r",
			Usage:       "randomly select single matching container from list of target containers",
			Destination: &actions.RandomMode,
		},
		cli.BoolFlag{
			Name:        "dry",
			Usage:       "does not create chaos, only logs planned chaos actions",
			Destination: &actions.DryMode,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func before(c *cli.Context) error {
	// set debug log level
	if c.GlobalBool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	// set log formatter to JSON
	if c.GlobalBool("json") {
		log.SetFormatter(&log.JSONFormatter{})
	}

	// set Slack log channel
	if c.GlobalString("slackhook") != "" {
		log.AddHook(&slackrus.SlackrusHook{
			HookURL:        c.GlobalString("slackhook"),
			AcceptedLevels: slackrus.LevelThreshold(log.GetLevel()),
			Channel:        c.GlobalString("slackchannel"),
			IconEmoji:      ":boar:",
			Username:       "pumba_bot",
		})
	}

	// Set-up container client
	tls, err := tlsConfig(c)
	if err != nil {
		return err
	}

	client = container.NewClient(c.GlobalString("host"), tls, !c.GlobalBool("no-pull"))

	handleSignals()
	return nil
}

func beforeCommand(c *cli.Context) error {
	// get recurrent time interval
	if intervalString := c.GlobalString("interval"); intervalString == "" {
		return errors.New("Undefined interval value.")
	} else if i, err := time.ParseDuration(intervalString); err != nil {
		return err
	} else {
		interval = i
	}

	// get container names or pattern: no Args means ALL containers
	if c.Args().Present() {
		// more than one argument, assume that this a list of names
		if len(c.Args()) > 1 {
			containerNames = c.Args()
			log.Debugf("Names: '%s'", containerNames)
		} else {
			first := c.Args().First()
			if strings.HasPrefix(first, re2prefix) {
				containerPattern = strings.Trim(first, re2prefix)
				log.Debugf("Pattern: '%s'", containerPattern)
			}
		}
	}

	return nil
}

// KILL Command
func kill(c *cli.Context) {
	// get signal
	signal := c.String("signal")
	if _, ok := LinuxSignals[signal]; !ok {
		log.Fatal(errors.New("Unexpected signal: " + signal))
	}
	// channel for 'kill' command
	dc := make(chan commandKill)
	// start interval timer
	ticker := time.NewTicker(interval)
	go func(cmd commandKill) {
		for range ticker.C {
			dc <- cmd
		}
	}(commandKill{signal})
	// handle 'kill' command
	chaos := actions.Pumba{}
	for cmd := range dc {
		wg.Add(1)
		go func(cmd commandKill) {
			defer wg.Done()
			if err := chaos.KillContainers(client, containerNames, containerPattern, cmd.signal); err != nil {
				log.Error(err)
			}
		}(cmd)
	}
}

// NETEM DELAY command
func netemDelay(c *cli.Context) {
	// get duration
	durationString := c.String("duration")
	if durationString == "" {
		log.Fatal(errors.New("Undefined duration interval value."))
	}
	duration, err := time.ParseDuration(durationString)
	if err != nil {
		log.Fatal(err)
	}
	// channel for 'netem' command
	dc := make(chan commandNetem)
	// start interval timer
	ticker := time.NewTicker(interval)
	go func(cmd commandNetem) {
		for range ticker.C {
			dc <- cmd
		}
	}(commandNetem{duration})
	// handle 'netem' command
	chaos := actions.Pumba{}
	for cmd := range dc {
		wg.Add(1)
		go func(cmd commandNetem) {
			defer wg.Done()
			var err error
			if containerPattern == "" {
				err = chaos.NetemByName(client, containerNames, defaultNetemCmd)
			} else {
				err = chaos.NetemByPattern(client, containerPattern, defaultNetemCmd)
			}
			if err != nil {
				log.Error(err)
			}
		}(cmd)
	}
}

// PAUSE command
func pause(c *cli.Context) {
	// get duration
	durationString := c.String("duration")
	if durationString == "" {
		log.Fatal(errors.New("Undefined duration interval value."))
	}
	duration, err := time.ParseDuration(durationString)
	if err != nil {
		log.Fatal(err)
	}
	// channel for 'pause' command
	dc := make(chan commandPause)
	// start interval timer
	ticker := time.NewTicker(interval)
	go func(cmd commandPause) {
		for range ticker.C {
			dc <- cmd
		}
	}(commandPause{duration})
	// handle 'pause' command
	chaos := actions.Pumba{}
	for cmd := range dc {
		wg.Add(1)
		go func(cmd commandPause) {
			defer wg.Done()
			var err error
			if containerPattern == "" {
				err = chaos.PauseByName(client, containerNames, cmd.duration)
			} else {
				err = chaos.PauseByPattern(client, containerPattern, cmd.duration)
			}
			if err != nil {
				log.Error(err)
			}
		}(cmd)
	}
}

// REMOVE Command
func remove(c *cli.Context) {
	// get force flag
	force := c.Bool("force")
	// get link flag
	link := c.String("link")
	// get link flag
	volumes := c.String("volumes")
	// channel for 'stop' command
	dc := make(chan commandRemove)
	// start interval timer
	ticker := time.NewTicker(interval)
	go func(cmd commandRemove) {
		for range ticker.C {
			dc <- cmd
		}
	}(commandRemove{force, link, volumes})
	// handle 'remove' command
	chaos := actions.Pumba{}
	for cmd := range dc {
		wg.Add(1)
		go func(cmd commandRemove) {
			defer wg.Done()
			if err := chaos.RemoveContainers(client, containerNames, containerPattern, cmd.force, cmd.link, cmd.volumes); err != nil {
				log.Error(err)
			}
		}(cmd)
	}
}

// STOP Command
func stop(c *cli.Context) {
	// get time to wait
	waitTime := c.Int("time")
	// channel for 'stop' command
	dc := make(chan commandStop)
	// start interval timer
	ticker := time.NewTicker(interval)
	go func(cmd commandStop) {
		for range ticker.C {
			dc <- cmd
		}
	}(commandStop{waitTime})
	// handle 'stop' command
	chaos := actions.Pumba{}
	for cmd := range dc {
		wg.Add(1)
		go func(cmd commandStop) {
			defer wg.Done()
			var err error
			if containerPattern == "" {
				err = chaos.StopByName(client, containerNames, cmd.waitTime)
			} else {
				err = chaos.StopByPattern(client, containerPattern, cmd.waitTime)
			}
			if err != nil {
				log.Error(err)
			}
		}(cmd)
	}
}

func handleSignals() {
	// Graceful shut-down on SIGINT/SIGTERM
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	go func() {
		<-c
		wg.Wait()
		os.Exit(1)
	}()
}

// tlsConfig translates the command-line options into a tls.Config struct
func tlsConfig(c *cli.Context) (*tls.Config, error) {
	var tlsConfig *tls.Config
	var err error
	caCertFlag := c.GlobalString("tlscacert")
	certFlag := c.GlobalString("tlscert")
	keyFlag := c.GlobalString("tlskey")

	if c.GlobalBool("tls") || c.GlobalBool("tlsverify") {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: !c.GlobalBool("tlsverify"),
		}

		// Load CA cert
		if caCertFlag != "" {
			var caCert []byte

			if strings.HasPrefix(caCertFlag, "/") {
				caCert, err = ioutil.ReadFile(caCertFlag)
				if err != nil {
					return nil, err
				}
			} else {
				caCert = []byte(caCertFlag)
			}

			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)

			tlsConfig.RootCAs = caCertPool
		}

		// Load client certificate
		if certFlag != "" && keyFlag != "" {
			var cert tls.Certificate

			if strings.HasPrefix(certFlag, "/") && strings.HasPrefix(keyFlag, "/") {
				cert, err = tls.LoadX509KeyPair(certFlag, keyFlag)
				if err != nil {
					return nil, err
				}
			} else {
				cert, err = tls.X509KeyPair([]byte(certFlag), []byte(keyFlag))
				if err != nil {
					return nil, err
				}
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	return tlsConfig, nil
}
