package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/alexei-led/pumba/pkg/action"
	"github.com/alexei-led/pumba/pkg/container"

	"github.com/alexei-led/pumba/pkg/chaos/docker/cmd"
	netemCmd "github.com/alexei-led/pumba/pkg/chaos/netem/cmd"

	log "github.com/sirupsen/logrus"

	"github.com/urfave/cli"

	"github.com/johntdyer/slackrus"
)

var (
	client     container.Client
	chaos      action.Chaos
	topContext context.Context
)

var (
	// Version that is passed on compile time through -ldflags
	Version = "built locally"

	// GitCommit that is passed on compile time through -ldflags
	GitCommit = "none"

	// GitBranch that is passed on compile time through -ldflags
	GitBranch = "none"

	// BuildTime that is passed on compile time through -ldflags
	BuildTime = "none"

	// HumanVersion is a human readable app version
	HumanVersion = fmt.Sprintf("%s - %.7s (%s) %s", Version, GitCommit, GitBranch, BuildTime)
)

const (
	// Re2Prefix re2 regexp string prefix
	Re2Prefix = "re2:"
	// DefaultInterface default network interface
	DefaultInterface = "eth0"
)

func contains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}
	_, ok := set[item]
	return ok
}

func init() {
	// set log level
	log.SetLevel(log.WarnLevel)
	log.SetFormatter(&log.TextFormatter{})
	// handle termination signal
	topContext = handleSignals()
}

func main() {
	rootCertPath := "/etc/ssl/docker"

	if os.Getenv("DOCKER_CERT_PATH") != "" {
		rootCertPath = os.Getenv("DOCKER_CERT_PATH")
	}

	app := cli.NewApp()
	app.Name = "Pumba"
	app.Version = HumanVersion
	app.Compiled = time.Now()
	app.Authors = []cli.Author{
		{
			Name:  "Alexei Ledenev",
			Email: "alexei.led@gmail.com",
		},
	}
	app.EnableBashCompletion = true
	app.Usage = "Pumba is a resilience testing tool, that helps applications tolerate random Docker container failures: process, network and performance."
	app.ArgsUsage = fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q)", Re2Prefix)
	app.Before = before
	app.Commands = []cli.Command{
		*cmd.NewKillCLICommand(topContext, client),
		*cmd.NewStopCLICommand(topContext, client),
		*cmd.NewPauseCLICommand(topContext, client),
		*cmd.NewRemoveCLICommand(topContext, client),
		{
			Name: "netem",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "duration, d",
					Usage: "network emulation duration; should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
				},
				cli.StringFlag{
					Name:  "interface, i",
					Usage: "network interface to apply delay on",
					Value: DefaultInterface,
				},
				cli.StringFlag{
					Name:  "target, t",
					Usage: "target IP filter; comma separated. netem will impact only on traffic to target IP(s)",
				},
				cli.StringFlag{
					Name:  "tc-image",
					Usage: "Docker image with tc (iproute2 package); try 'gaiadocker/iproute2'",
				},
			},
			Usage:       "emulate the properties of wide area networks",
			ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", Re2Prefix),
			Description: "delay, loss, duplicate and re-order (run 'netem') packets, and limit the bandwidth, to emulate different network problems",
			Subcommands: []cli.Command{
				*netemCmd.NewDelayCLICommand(topContext, client),
				*netemCmd.NewLossCLICommand(topContext, client),
				*netemCmd.NewLossStateCLICommand(topContext, client),
				*netemCmd.NewLossGECLICommand(topContext, client),
				{
					Name:  "duplicate",
					Usage: "TBD",
				},
				{
					Name: "corrupt",

					Usage: "TBD",
				},
				{
					Name: "rate",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "rate, r",
							Usage: "delay outgoing packets; in common units",
							Value: "100kbit",
						},
						cli.IntFlag{
							Name:  "packetoverhead, p",
							Usage: "per packet overhead; in bytes",
							Value: 0,
						},
						cli.IntFlag{
							Name:  "cellsize, s",
							Usage: "cell size of the simulated link layer scheme",
							Value: 0,
						},
						cli.IntFlag{
							Name:  "celloverhead, c",
							Usage: "per cell overhead; in bytes",
							Value: 0,
						},
					},
					Usage:       "rate limit egress traffic",
					ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", Re2Prefix),
					Description: "rate limit egress traffic for specified containers",
					Action:      netemRate,
				},
			},
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
		cli.StringFlag{
			Name:   "log-level, l",
			Usage:  "set log level (debug, info, warning(*), error, fatal, panic)",
			Value:  "warning",
			EnvVar: "LOG_LEVEL",
		},
		cli.BoolFlag{
			Name:   "json, j",
			Usage:  "produce log in JSON format: Logstash and Splunk friendly",
			EnvVar: "LOG_JSON",
		},
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
			Destination: &action.RandomMode,
		},
		cli.BoolFlag{
			Name:        "dry-run",
			Usage:       "dry run does not create chaos, only logs planned chaos commands",
			Destination: &action.DryMode,
			EnvVar:      "DRY-RUN",
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func before(c *cli.Context) error {
	// set debug log level
	switch level := c.GlobalString("log-level"); level {
	case "debug", "DEBUG":
		log.SetLevel(log.DebugLevel)
	case "info", "INFO":
		log.SetLevel(log.InfoLevel)
	case "warning", "WARNING":
		log.SetLevel(log.WarnLevel)
	case "error", "ERROR":
		log.SetLevel(log.ErrorLevel)
	case "fatal", "FATAL":
		log.SetLevel(log.FatalLevel)
	case "panic", "PANIC":
		log.SetLevel(log.PanicLevel)
	default:
		log.SetLevel(log.WarnLevel)
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
	// create new Chaos instance
	chaos = action.NewChaos()
	return nil
}

func getIntervalValue(c *cli.Context) (time.Duration, error) {
	// get recurrent time interval
	if intervalString := c.GlobalString("interval"); intervalString == "" {
		log.Debug("no interval specified, running only once")
		return 0, nil
	} else if interval, err := time.ParseDuration(intervalString); err == nil {
		return interval, nil
	} else {
		return 0, err
	}
}

func getNamesOrPattern(c *cli.Context) ([]string, string) {
	names := []string{}
	pattern := ""
	// get container names or pattern: no Args means ALL containers
	if c.Args().Present() {
		// more than one argument, assume that this a list of names
		if len(c.Args()) > 1 {
			names = c.Args()
			log.Debugf("Names: '%s'", names)
		} else {
			first := c.Args().First()
			if strings.HasPrefix(first, Re2Prefix) {
				pattern = strings.Trim(first, Re2Prefix)
				log.Debugf("Pattern: '%s'", pattern)
			} else {
				names = append(names, first)
				log.Debugf("Names: '%s'", names)
			}
		}
	}
	return names, pattern
}

func runChaosCommand(cmd interface{}, interval time.Duration, names []string, pattern string, chaosFn func(context.Context, container.Client, []string, string, interface{}) error) {
	// create Time channel for specified interval
	var tick <-chan time.Time
	if interval == 0 {
		tick = time.NewTimer(interval).C
	} else {
		tick = time.NewTicker(interval).C
	}

	// handle the 'chaos' command
	ctx, cancel := context.WithCancel(topContext)
	for {
		// cancel current context on exit
		defer cancel()
		// run chaos function
		if err := chaosFn(ctx, client, names, pattern, cmd); err != nil {
			log.Error(err)
		}
		// wait for next timer tick or cancel
		select {
		case <-topContext.Done():
			return // not to leak the goroutine
		case <-tick:
			if interval == 0 {
				return // not to leak the goroutine
			}
			log.Debug("Next chaos execution (tick) ...")
		}
	}
}

func parseNetemOptions(c *cli.Context) ([]string, string, time.Duration, string, []net.IP, string, error) {
	// get names or pattern
	names, pattern := getNamesOrPattern(c)
	// get interval
	interval, err := getIntervalValue(c)
	if err != nil {
		log.Error(err)
		return names, pattern, 0, "", nil, "", err
	}
	// get duration
	var durationString string
	if c.Parent() != nil {
		durationString = c.Parent().String("duration")
	}
	if durationString == "" {
		err = errors.New("Undefined duration interval")
		log.Error(err)
		return names, pattern, 0, "", nil, "", err
	}
	duration, err := time.ParseDuration(durationString)
	if err != nil {
		log.Error(err)
		return names, pattern, 0, "", nil, "", err
	}
	if interval != 0 && duration >= interval {
		err = errors.New("Duration cannot be bigger than interval")
		log.Error(err)
		return names, pattern, 0, "", nil, "", err
	}
	// get network interface and target ip(s)
	netInterface := DefaultInterface
	var ips []net.IP
	if c.Parent() != nil {
		netInterface = c.Parent().String("interface")
		// protect from Command Injection, using Regexp
		reInterface := regexp.MustCompile("[a-zA-Z]+[0-9]{0,2}")
		validInterface := reInterface.FindString(netInterface)
		if netInterface != validInterface {
			err = fmt.Errorf("Bad network interface name. Must match '%s'", reInterface.String())
			log.Error(err)
			return names, pattern, duration, "", nil, "", err
		}
		// get target IP Filter
		target := c.Parent().String("target")
		if target != "" {
			for _, str := range strings.Split(target, ",") {
				ip := net.ParseIP(str)
				if ip == nil {
					err = fmt.Errorf("Bad target specification. could not parse '%s' as an ip", str)
					log.Error(err)
					return names, pattern, duration, "", ips, "", err
				}
				ips = append(ips, ip)
			}
		}
	}
	// get Docker image with tc (iproute2 package)
	var image string
	if c.Parent() != nil {
		image = c.Parent().String("tc-image")
	}
	return names, pattern, duration, netInterface, ips, image, nil
}

// NETEM RATE command
func netemRate(c *cli.Context) error {
	// get interval
	interval, err := getIntervalValue(c)
	if err != nil {
		return err
	}
	// parse common netem options
	names, pattern, duration, netInterface, ips, image, err := parseNetemOptions(c)
	if err != nil {
		return err
	}
	// get target egress rate
	rateString := c.String("rate")
	if rateString == "" {
		err = errors.New("Undefined rate limit")
		log.Error(err)
		return err
	}
	rate, err := parseRate(rateString)
	if err != nil {
		log.Error(err)
		return err
	}
	// get packet overhead
	packetOverhead := c.Int("packetoverhead")
	// get cell size
	cellSize := c.Int("cellsize")
	if cellSize < 0 {
		err = errors.New("Invalid cell size: must be a non-negative integer")
		log.Error(err)
		return err
	}
	// get cell overhead
	cellOverhead := c.Int("celloverhead")
	// pepare netem rate command
	rateCmd := action.CommandNetemRate{
		NetInterface:   netInterface,
		IPs:            ips,
		Duration:       duration,
		Rate:           rate,
		PacketOverhead: packetOverhead,
		CellSize:       cellSize,
		CellOverhead:   cellOverhead,
		Image:          image,
	}
	runChaosCommand(rateCmd, interval, names, pattern, chaos.NetemRateContainers)
	return nil
}

func handleSignals() context.Context {
	// Graceful shut-down on SIGINT/SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// create cancelable context
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()
		sid := <-sig
		log.Debugf("Received signal: %d", sid)
		log.Debug("Canceling running chaos commands ...")
		log.Debug("Gracefully exiting after some cleanup ...")
	}()

	return ctx
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

// Parse rate
func parseRate(rate string) (string, error) {
	reRate := regexp.MustCompile("[0-9]+[gmk]?bit")
	validRate := reRate.FindString(rate)
	if rate != validRate {
		err := fmt.Errorf("Invalid rate. Must match '%s'", reRate.String())
		log.Error(err)
		return "", err
	}
	return rate, nil
}
