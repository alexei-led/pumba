package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	ctr "github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	topContext context.Context

	// runtimeClient is captured by before() once createRuntimeClient succeeds and
	// then read by every CLI builder via the chaos.Runtime closure passed to
	// initializeCLICommands. app.After calls Close on the same value.
	runtimeClient ctr.Client
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
	topContext = handleSignals() //nolint:fatcontext // top-level process signal context, assigned once in init
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
	app.After = func(_ *cli.Context) error {
		if runtimeClient != nil {
			return runtimeClient.Close()
		}
		return nil
	}
	app.Commands = initializeCLICommands(func() ctr.Client { return runtimeClient })
	app.Flags = globalFlags(rootCertPath)

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func before(c *cli.Context) error {
	setupLogging(cliflags.NewV1FromApp(c))
	client, err := createRuntimeClient(c)
	if err != nil {
		return err
	}
	runtimeClient = client
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
