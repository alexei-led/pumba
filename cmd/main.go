package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/docker/cmd"
	ipTablesCmd "github.com/alexei-led/pumba/pkg/chaos/iptables/cmd"
	netemCmd "github.com/alexei-led/pumba/pkg/chaos/netem/cmd"
	stressCmd "github.com/alexei-led/pumba/pkg/chaos/stress/cmd"
	"github.com/alexei-led/pumba/pkg/runtime/containerd"
	"github.com/alexei-led/pumba/pkg/runtime/docker"
	"github.com/johntdyer/slackrus"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	topContext context.Context
)

var (
	// version that is passed on compile time through -ldflags
	version = "local"

	// commit that is passed on compile time through -ldflags
	commit = "none"

	// branch that is passed on compile time through -ldflags
	branch = "none"

	// buildTime that is passed on compile time through -ldflags
	buildTime = "none"

	// versionSingature is a human readable app version
	versionSingature = fmt.Sprintf("%s - [%s:%.7s] %s", version, branch, commit, buildTime)
)

const (
	// re2 regexp string prefix
	re2Prefix = "re2:"
	// default network interface
	defaultInterface = "eth0"
)

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
	app.Version = versionSingature
	app.Compiled = time.Now()
	app.Authors = []cli.Author{
		{
			Name:  "Alexei Ledenev",
			Email: "alexei.led@gmail.com",
		},
	}
	app.EnableBashCompletion = true
	app.Usage = "Pumba is a resilience testing tool, that helps applications tolerate random Docker container failures: process, network and performance."
	app.ArgsUsage = fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q)", re2Prefix)
	app.Before = before
	app.Commands = initializeCLICommands()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host, H",
			Usage:  "daemon socket to connect to",
			Value:  "unix:///var/run/docker.sock",
			EnvVar: "DOCKER_HOST",
		},
		cli.StringFlag{
			Name:  "runtime",
			Usage: "container runtime (docker, containerd)",
			Value: "docker",
		},
		cli.StringFlag{
			Name:  "containerd-socket",
			Usage: "containerd socket location",
			Value: "/run/containerd/containerd.sock",
		},
		cli.StringFlag{
			Name:  "containerd-namespace",
			Usage: "containerd namespace",
			Value: "k8s.io",
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
		cli.DurationFlag{
			Name:  "interval, i",
			Usage: "recurrent interval for chaos command; use with optional unit suffix: 'ms/s/m/h'",
		},
		cli.StringSliceFlag{
			Name:  "label",
			Usage: "filter containers by labels, e.g. '--label key=value' (use '--label k1=v1 --label k2=v2' or '--label k1=v1,k2=v2' for multiple, AND logic)",
		},
		cli.BoolFlag{
			Name:  "random, r",
			Usage: "randomly select single matching container from list of target containers",
		},
		cli.BoolFlag{
			Name:   "dry-run",
			Usage:  "dry run does not create chaos, only logs planned chaos commands",
			EnvVar: "DRY-RUN",
		},
		cli.BoolFlag{
			Name:  "skip-error",
			Usage: "skip chaos command error and retry to execute the command on next interval tick",
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
	var err error
	switch runtime := c.GlobalString("runtime"); runtime {
	case "docker":
		tlsCfg, err := tlsConfig(c)
		if err != nil {
			return err
		}
		// create new Docker client
		chaos.DockerClient, err = docker.NewClient(c.GlobalString("host"), tlsCfg)
		if err != nil {
			return fmt.Errorf("could not create Docker client: %w", err)
		}
	case "containerd":
		socket := c.GlobalString("containerd-socket")
		namespace := c.GlobalString("containerd-namespace")
		chaos.DockerClient, err = containerd.NewClient(socket, namespace)
		if err != nil {
			return fmt.Errorf("could not create containerd client: %w", err)
		}
	default:
		return fmt.Errorf("unsupported runtime: %s", runtime)
	}

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
		log.Debugf("Received signal: %d\n", sid)
		log.Debug("Canceling running chaos commands ...")
		log.Debug("Gracefully exiting after some cleanup ...")
	}()

	return ctx
}

// tlsConfig translates the command-line options into a tls.Config struct
func tlsConfig(c *cli.Context) (*tls.Config, error) {
	var tlsCfg *tls.Config
	var err error
	caCertFlag := c.GlobalString("tlscacert")
	certFlag := c.GlobalString("tlscert")
	keyFlag := c.GlobalString("tlskey")

	if c.GlobalBool("tls") || c.GlobalBool("tlsverify") {
		tlsCfg = &tls.Config{
			InsecureSkipVerify: !c.GlobalBool("tlsverify"), //nolint:gosec
		}

		// Load CA cert
		if caCertFlag != "" {
			var caCert []byte
			if strings.HasPrefix(caCertFlag, "/") {
				caCert, err = os.ReadFile(caCertFlag)
				if err != nil {
					return nil, fmt.Errorf("unable to read CA certificate: %w", err)
				}
			} else {
				caCert = []byte(caCertFlag)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsCfg.RootCAs = caCertPool
		}

		// Load client certificate
		if certFlag != "" && keyFlag != "" {
			var cert tls.Certificate
			if strings.HasPrefix(certFlag, "/") && strings.HasPrefix(keyFlag, "/") {
				cert, err = tls.LoadX509KeyPair(certFlag, keyFlag)
				if err != nil {
					return nil, fmt.Errorf("unable to load client certificate: %w", err)
				}
			} else {
				cert, err = tls.X509KeyPair([]byte(certFlag), []byte(keyFlag))
				if err != nil {
					return nil, fmt.Errorf("unable to load client certificate: %w", err)
				}
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
	}
	return tlsCfg, nil
}

//nolint:funlen
func initializeCLICommands() []cli.Command {
	return []cli.Command{
		*cmd.NewKillCLICommand(topContext),
		*cmd.NewExecCLICommand(topContext),
		*cmd.NewRestartCLICommand(topContext),
		*cmd.NewStopCLICommand(topContext),
		*cmd.NewPauseCLICommand(topContext),
		*cmd.NewRemoveCLICommand(topContext),
		*stressCmd.NewStressCLICommand(topContext),
		{
			Name: "netem",
			Flags: []cli.Flag{
				cli.DurationFlag{
					Name:  "duration, d",
					Usage: "network emulation duration; should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
				},
				cli.StringFlag{
					Name:  "interface, i",
					Usage: "network interface to apply delay on",
					Value: defaultInterface,
				},
				cli.StringSliceFlag{
					Name:  "target, t",
					Usage: "target IP filter; supports multiple IPs; supports CIDR notation",
				},
				cli.StringFlag{
					Name:  "egress-port, egressPort",
					Usage: "target port filter for egress, or sport; supports multiple ports (comma-separated)",
				},
				cli.StringFlag{
					Name:  "ingress-port, ingressPort",
					Usage: "target port filter for ingress, or dport; supports multiple ports (comma-separated)",
				},
				cli.StringFlag{
					Name:  "tc-image",
					Usage: "Docker image with tc (iproute2 package) and iptables",
					Value: "ghcr.io/alexei-led/pumba-alpine-nettools:latest",
				},
				cli.BoolTFlag{
					Name:  "pull-image",
					Usage: "force pull tc-image",
				},
			},
			Usage:       "emulate the properties of wide area networks",
			ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", re2Prefix),
			Description: "delay, loss, duplicate and re-order (run 'netem') packets, and limit the bandwidth, to emulate different network problems",
			Subcommands: []cli.Command{
				*netemCmd.NewDelayCLICommand(topContext),
				*netemCmd.NewLossCLICommand(topContext),
				*netemCmd.NewLossStateCLICommand(topContext),
				*netemCmd.NewLossGECLICommand(topContext),
				*netemCmd.NewRateCLICommand(topContext),
				*netemCmd.NewDuplicateCLICommand(topContext),
				*netemCmd.NewCorruptCLICommand(topContext),
			},
		},
		{
			Name: "iptables",
			Flags: []cli.Flag{
				cli.DurationFlag{
					Name:  "duration, d",
					Usage: "network emulation duration; should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
				},
				cli.StringFlag{
					Name:  "interface, i",
					Usage: "network interface to apply input rules on",
					Value: defaultInterface,
				},
				cli.StringFlag{
					Name:  "protocol, p",
					Usage: "protocol to apply input rules on (any, udp, tcp or icmp)",
					Value: "any",
				},
				cli.StringSliceFlag{
					Name:  "source, src, s",
					Usage: "source IP filter; supports multiple IPs; supports CIDR notation",
				},
				cli.StringSliceFlag{
					Name:  "destination, dest",
					Usage: "destination IP filter; supports multiple IPs; supports CIDR notation",
				},
				cli.StringFlag{
					Name:  "src-port, sport",
					Usage: "source port filter; supports multiple ports (comma-separated)",
				},
				cli.StringFlag{
					Name:  "dst-port, dport",
					Usage: "destination port filter; supports multiple ports (comma-separated)",
				},
				cli.StringFlag{
					Name:  "iptables-image",
					Usage: "Docker image with iptables and tc (iproute2 package)",
					Value: "ghcr.io/alexei-led/pumba-alpine-nettools:latest",
				},
				cli.BoolTFlag{
					Name:  "pull-image",
					Usage: "force pull iptables-image",
				},
			},
			Usage:       "apply IPv4 packet filter on incoming IP packets",
			ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", re2Prefix),
			Description: "emulate loss of incoming packets, all ports and address arguments will result in separate rules",
			Subcommands: []cli.Command{
				*ipTablesCmd.NewLossCLICommand(topContext),
			},
		},
	}
}
