package main

import (
	"fmt"

	"github.com/urfave/cli"
)

func globalFlags(rootCertPath string) []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:   "host, H",
			Usage:  "daemon socket to connect to",
			Value:  "unix:///var/run/docker.sock",
			EnvVar: "DOCKER_HOST",
		},
		cli.StringFlag{
			Name:  "runtime",
			Usage: "container runtime (docker, containerd, podman)",
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
		cli.StringFlag{
			Name:  "podman-socket",
			Usage: "Podman socket URI (auto-detected if empty; e.g. unix:///run/podman/podman.sock)",
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
}
