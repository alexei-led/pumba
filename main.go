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
	"net"

	"github.com/gaia-adm/pumba/action"
	"github.com/gaia-adm/pumba/container"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/johntdyer/slackrus"
)

var (
	wg      sync.WaitGroup
	client  container.Client
	cleanup bool
)

const (
	defaultKillSignal = "SIGKILL"
	re2prefix         = "re2:"
	release           = "v0.1.10"
	defaultNetemCmd   = "delay 1000ms"
)

type commandT struct {
	pattern string
	names   []string
	command string
	option string
}

func init() {
	log.SetLevel(log.InfoLevel)
	log.SetFormatter(&log.TextFormatter{})
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func main() {
	rootCertPath := "/etc/ssl/docker"

	if os.Getenv("DOCKER_CERT_PATH") != "" {
		rootCertPath = os.Getenv("DOCKER_CERT_PATH")
	}

	app := cli.NewApp()
	app.Name = "Pumba"
	app.Version = release
	app.Usage = "Pumba is a resiliency tool that helps applications tolerate random Docker container failures."
	app.Before = before
	app.Commands = []cli.Command{
		{
			Name: "run",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:  "chaos, c",
					Usage: "chaos command: `container(s,)/re2:regex|interval(s/m/h postfix)|chaos_command(see above)`",
				},
				cli.BoolFlag{
					Name:        "random, r",
					Usage:       "Random mode: randomly select single matching container as a target for the specified chaos action",
					Destination: &actions.RandomMode,
				},
				cli.BoolFlag{
					Name:        "dry",
					Usage:       "enable 'dry run' mode: does not execute chaos action, just logs actions",
					Destination: &actions.DryMode,
				},
			},
			Usage: "Pumba starts making chaos: periodically (and randomly) affecting specified containers.",
			Description: "Ask Pumba to run periodically (and randomly) specified chaos_command on selected container(s).\n\n" +
				"   List of supported chaos_command(s):\n" +
				"    * STOP - stop running container(s)\n" +
				"    * KILL(:SIGNAL) - kill running container(s), optionally sending specified Linux SIGNAL (SIGKILL by default)\n" +
				"    * RM - force remove running container(s)\n" +
				"    * PAUSE:interval(ms/s/m/h postfix) - pause all processes within running container(s) for specified interval" + 
				"    * DISRUPT(:netem command)(:target ip)",
			Action: run,
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

	cleanup = c.GlobalBool("cleanup")

	// Set-up container client
	tls, err := tlsConfig(c)
	if err != nil {
		return err
	}

	client = container.NewClient(c.GlobalString("host"), tls, !c.GlobalBool("no-pull"))

	handleSignals()
	return nil
}

func run(c *cli.Context) {
	if err := actions.CheckPrereqs(client, cleanup); err != nil {
		log.Fatal(err)
	}
	if err := createChaos(actions.Pumba{}, c.StringSlice("chaos"), 1, false); err != nil {
		log.Fatal(err)
	}
}

func createChaos(chaos actions.Chaos, args []string, limit int, test bool) error {
	// docker channel to pass all "stop" commands to
	dc := make(chan commandT)
	glimit := limit * len(args)

	// range over all chaos arguments
	for _, chaosArg := range args {
		s := strings.Split(chaosArg, "|")
		if len(s) != 3 {
			return errors.New("Unexpected format for chaos_arg: use | separated triple")
		}
		// get container name pattern
		var pattern string
		var names []string
		if strings.HasPrefix(s[0], re2prefix) {
			pattern = strings.Trim(s[0], re2prefix)
			log.Debugf("Pattern: '%s'", pattern)
		} else {
			if s[0] != "" {
				names = strings.Split(s[0], ",")
			}
			log.Debugf("Names: '%s'", names)
		}
		// get interval duration
		interval, err := time.ParseDuration(s[1])
		if err != nil {
			return err
		}
		log.Debugf("Interval: '%s'", interval.String())
		// get command and its option (if specified)
		cs := strings.Split(s[2], ":")
		command := strings.ToUpper(cs[0])
		if !stringInSlice(command, []string{"STOP", "KILL", "RM", "PAUSE", "DISRUPT"}) {
			return errors.New("Unexpected command in chaos option: can be STOP, KILL, RM, PAUSE or DISRUPT")
		}
		log.Debugf("Command: '%s'", command)
		// 2 actions upport a second argument: KILL/STOP:signal 
		//	and DISRUPT:netem command:target ip
		// accordingly assign 2nd cmd line argument if exists
		option := defaultKillSignal
		if command == "DISRUPT" {
			option = defaultNetemCmd
		}
		if len(cs) >= 2 {
			option = cs[1]
			if command == "PAUSE" {
				log.Debugf("Pause interval: '%s'", option)
			} else if command == "STOP" || command == "KILL" {
				// convert signal to UPPER
				option := strings.ToUpper(option)
				log.Debugf("Signal: '%s'", option)
			} else if command == "DISRUPT" {
				option = defaultNetemCmd
				// the string may be netem command or target IP - as the user
				//  can omit the command part and use the default
				if len(cs) == 3 {
					// then we have both command and target IP, just need to put them 
					//   together for the internal implementaion
					option = cs[1] + ":" + cs[2]
				} else {
					// If it's IP, re-concat it with the default command
					//  otherwise, just replace the netem command
					if net.ParseIP(cs[1]) != nil {
						option = option + ":" + cs[1]
					} else {
						option = cs[1]
					}
				}
				log.Debugf("Netem Command: '%s'", option)
			} else {
				log.Debugf("2nd argument doesn't correspond with command: '%s'", cs[1])	
				return errors.New("Surplus 2nd argument to chaos action command")
			}
		}

		ticker := time.NewTicker(interval)
		go func(cmd commandT, limit int, test bool) {
			for range ticker.C {
				if limit > 0 {
					log.Debugf("Tick: '%s'", cmd)
					dc <- cmd
					if test {
						limit--
					}
				}
			}
		}(commandT{pattern, names, command, option}, limit, test)
	}
	for cmd := range dc {
		if test {
			glimit--
		}
		if glimit == 0 {
			break
		}
		wg.Add(1)
		go func(cmd commandT) {
			defer wg.Done()
			var err error
			switch cmd.command {
			case "STOP":
				if cmd.pattern == "" {
					err = chaos.StopByName(client, cmd.names)
				} else {
					err = chaos.StopByPattern(client, cmd.pattern)
				}
			case "KILL":
				if cmd.pattern == "" {
					err = chaos.KillByName(client, cmd.names, cmd.option)
				} else {
					err = chaos.KillByPattern(client, cmd.pattern, cmd.option)
				}
			case "RM":
				if cmd.pattern == "" {
					err = chaos.RemoveByName(client, cmd.names, true)
				} else {
					err = chaos.RemoveByPattern(client, cmd.pattern, true)
				}
			case "DISRUPT":
				if cmd.pattern == "" {
					err = chaos.DisruptByName(client, cmd.names, cmd.option)
				} else {
					err = chaos.DisruptByPattern(client, cmd.pattern,cmd.option)
				}
			case "PAUSE":
				if cmd.pattern == "" {
					err = chaos.PauseByName(client, cmd.names, cmd.option)
				} else {
					err = chaos.PauseByPattern(client, cmd.pattern, cmd.option)
				}
			}
			if err != nil {
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
