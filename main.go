package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"regexp"
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
	containerNames   []string
	containerPattern string
	chaos            actions.Chaos
	commandTimeChan  <-chan time.Time
	testRun          bool
	reInterface      *regexp.Regexp
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
	release       = "v0.2.0"
	defaultSignal = "SIGKILL"
	re2prefix     = "re2:"
)

type commandKill struct {
	signal string
}

type commandPause struct {
	duration time.Duration
}

type commandNetemDelay struct {
	netInterface string
	duration     time.Duration
	amount       int
	variation    int
	correlation  int
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
					Usage: "termination signal, that will be sent by Pumba to the main process inside target container(s)",
					Value: defaultSignal,
				},
			},
			Usage:       "kill specified containers",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "send termination signal to the main process inside target container(s)",
			Action:      kill,
			Before:      beforeCommand,
		},
		{
			Name: "netem",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "duration, d",
					Usage: "network emulation duration; should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
				},
			},
			Usage:       "emulate the properties of wide area networks",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "delay, loss, duplicate and re-order (run 'netem') packets, to emulate different network problems",
			Before:      beforeCommand,
			Subcommands: []cli.Command{
				{
					Name: "delay",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "interface, i",
							Usage: "network interface to apply delay on",
							Value: "eth0",
						},
						cli.IntFlag{
							Name:  "amount, a",
							Usage: "delay amount; in milliseconds",
							Value: 100,
						},
						cli.IntFlag{
							Name:  "variation, v",
							Usage: "random delay variation; in milliseconds; example: 100ms ± 10ms",
							Value: 10,
						},
						cli.IntFlag{
							Name:  "correlation, c",
							Usage: "delay correlation; in percents",
							Value: 20,
						},
					},
					Usage:       "dealy egress traffic",
					ArgsUsage:   "containers (name, list of names, RE2 regex)",
					Description: "dealy egress traffic for specified containers; networks show variability so it is possible to add random variation; delay variation isn't purely random, so to emulate that there is a correlation",
					Action:      netemDelay,
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
					Usage: "pause duration: should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
				},
			},
			Usage:       "pause all processes",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "pause all running processes within target containers",
			Action:      pause,
			Before:      beforeCommand,
		},
		{
			Name: "stop",
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "time, t",
					Usage: "seconds to wait for stop before killing container (default 10)",
					Value: 10,
				},
			},
			Usage:       "stop containers",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "stop the main process inside target containers, sending  SIGTERM, and then SIGKILL after a grace period",
			Action:      stop,
			Before:      beforeCommand,
		},
		{
			Name: "rm",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "force, f",
					Usage: "force the removal of a running container (with SIGKILL)",
				},
				cli.StringFlag{
					Name:  "link, l",
					Usage: "remove the specified link",
				},
				cli.StringFlag{
					Name:  "volumes, v",
					Usage: "remove the volumes associated with the container",
				},
			},
			Usage:       "remove containers",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "remove target containers, with links and voluems",
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
			Usage: "web hook url; send Pumba log events to Slack",
		},
		cli.StringFlag{
			Name:  "slackchannel",
			Usage: "Slack channel (default #pumba)",
			Value: "#pumba",
		},
		cli.StringFlag{
			Name:  "interval, i",
			Usage: "recurrent interval for chaos command; use with optional unit suffix: 'ms/s/m/h'",
		},
		cli.BoolFlag{
			Name:        "random, r",
			Usage:       "randomly select single matching container from list of target containers",
			Destination: &actions.RandomMode,
		},
		cli.BoolFlag{
			Name:        "dry",
			Usage:       "dry runl does not create chaos, only logs planned chaos commands",
			Destination: &actions.DryMode,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func before(c *cli.Context) error {
	// set chaos to Pumba{}
	chaos = actions.Pumba{}
	// network interface name valid regexp
	reInterface = regexp.MustCompile("[a-zA-Z]+[0-9]{0,2}")
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
	// create new Docker client
	client = container.NewClient(c.GlobalString("host"), tls)
	// habdle termination signal
	handleSignals()
	return nil
}

// beforeCommand run before each chaos command
func beforeCommand(c *cli.Context) error {
	// get recurrent time interval
	if intervalString := c.GlobalString("interval"); intervalString == "" {
		return errors.New("Undefined interval value.")
	} else if interval, err := time.ParseDuration(intervalString); err != nil {
		return err
	} else {
		// create Ticker Time channel for specified intterval
		ticker := time.NewTicker(interval)
		commandTimeChan = ticker.C
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
func kill(c *cli.Context) error {
	// get signal
	signal := c.String("signal")
	if _, ok := LinuxSignals[signal]; !ok {
		err := errors.New("Unexpected signal: " + signal)
		log.Error(err)
		return err
	}
	// channel for 'kill' command
	dc := make(chan commandKill)
	// handle interval timer event
	go func(cmd commandKill) {
		for range commandTimeChan {
			dc <- cmd
			if testRun {
				close(dc)
			}
		}
	}(commandKill{signal})
	// handle 'kill' command
	for cmd := range dc {
		wg.Add(1)
		go func(cmd commandKill) {
			defer wg.Done()
			if err := chaos.KillContainers(client, containerNames, containerPattern, cmd.signal); err != nil {
				log.Error(err)
			}
		}(cmd)
	}
	return nil
}

// NETEM DELAY command
func netemDelay(c *cli.Context) error {
	// get duration
	var durationString string
	if c.Parent() != nil {
		durationString = c.Parent().String("duration")
	}
	if durationString == "" {
		err := errors.New("Undefined duration interval")
		log.Error(err)
		return err
	}
	duration, err := time.ParseDuration(durationString)
	if err != nil {
		log.Error(err)
		return err
	}
	// get network interface
	netInterface := "eth0"
	if c.Parent() != nil {
		netInterface = c.Parent().String("interface")
		// protect from Command Injection, using Regexp
		netInterface = reInterface.FindString(netInterface)
	}
	// get delay amount
	amount := c.Int("amount")
	if amount <= 0 {
		err = errors.New("Invalid delay amount")
		log.Error(err)
		return err
	}
	// get delay variation
	variation := c.Int("variation")
	if variation < 0 {
		err = errors.New("Invalid delay variation")
		log.Error(err)
		return err
	}
	// get delay variation
	correlation := c.Int("correlation")
	if correlation < 0 || correlation > 100 {
		err = errors.New("Invalid delay correlation: must be between 0 and 100")
		log.Error(err)
		return err
	}
	// channel for 'netem' command
	dc := make(chan commandNetemDelay)
	// handle interval timer event
	go func(cmd commandNetemDelay) {
		for range commandTimeChan {
			dc <- cmd
			if testRun {
				close(dc)
			}
		}
	}(commandNetemDelay{netInterface, duration, amount, variation, correlation})
	// handle 'netem' command
	for cmd := range dc {
		wg.Add(1)
		go func(cmd commandNetemDelay) {
			defer wg.Done()
			if err := chaos.NetemDelayContainers(client, containerNames, containerPattern, cmd.netInterface, cmd.duration, cmd.amount, cmd.variation, cmd.correlation); err != nil {
				log.Error(err)
			}
		}(cmd)
	}
	return nil
}

// PAUSE command
func pause(c *cli.Context) error {
	// get duration
	durationString := c.String("duration")
	if durationString == "" {
		err := errors.New("Undefined duration interval")
		log.Error(err)
		return err
	}
	duration, err := time.ParseDuration(durationString)
	if err != nil {
		log.Error(err)
		return err
	}
	// channel for 'pause' command
	dc := make(chan commandPause)
	// handle interval timer event
	go func(cmd commandPause) {
		for range commandTimeChan {
			dc <- cmd
			if testRun {
				close(dc)
			}
		}
	}(commandPause{duration})
	// handle 'pause' command
	for cmd := range dc {
		wg.Add(1)
		go func(cmd commandPause) {
			defer wg.Done()
			if err := chaos.PauseContainers(client, containerNames, containerPattern, cmd.duration); err != nil {
				log.Error(err)
			}
		}(cmd)
	}
	return nil
}

// REMOVE Command
func remove(c *cli.Context) error {
	// get force flag
	force := c.Bool("force")
	// get link flag
	link := c.String("link")
	// get link flag
	volumes := c.String("volumes")
	// channel for 'stop' command
	dc := make(chan commandRemove)
	// handle interval timer event
	go func(cmd commandRemove) {
		for range commandTimeChan {
			dc <- cmd
			if testRun {
				close(dc)
			}
		}
	}(commandRemove{force, link, volumes})
	// handle 'remove' command
	for cmd := range dc {
		wg.Add(1)
		go func(cmd commandRemove) {
			defer wg.Done()
			if err := chaos.RemoveContainers(client, containerNames, containerPattern, cmd.force, cmd.link, cmd.volumes); err != nil {
				log.Error(err)
			}
		}(cmd)
	}
	return nil
}

// STOP Command
func stop(c *cli.Context) error {
	// get time to wait
	waitTime := c.Int("time")
	// channel for 'stop' command
	dc := make(chan commandStop)
	// handle interval timer event
	go func(cmd commandStop) {
		for range commandTimeChan {
			dc <- cmd
			if testRun {
				close(dc)
			}
		}
	}(commandStop{waitTime})
	// handle 'stop' command
	for cmd := range dc {
		wg.Add(1)
		go func(cmd commandStop) {
			defer wg.Done()
			if err := chaos.StopContainers(client, containerNames, containerPattern, cmd.waitTime); err != nil {
				log.Error(err)
			}
		}(cmd)
	}
	return nil
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
