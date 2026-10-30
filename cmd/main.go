package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
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
	if code := run(os.Args, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

// run executes the CLI and returns the process exit code: 1 exactly when the
// command failed, in every --output mode. In ndjson mode run.end is the last
// stdout line.
func run(args []string, stdout, stderr io.Writer) int {
	out := newEventOutput(stdout, stderr, version)
	err := newApp(out).Run(args)
	if err != nil {
		log.StandardLogger().Log(log.FatalLevel, err)
	}
	flushSlackHooks()
	return out.finish(err, topContext.Err() != nil)
}

func newApp(out *eventOutput) *cli.App {
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
	app.ArgsUsage = fmt.Sprintf("containers (names and RE2 regexes prefixed with %q; may be mixed)", re2Prefix)
	app.Writer = out.stdout
	app.ErrWriter = out.stderr
	app.Before = func(c *cli.Context) error { return before(c, out) }
	app.After = func(_ *cli.Context) error {
		if runtimeClient != nil {
			return runtimeClient.Close()
		}
		return nil
	}
	app.OnUsageError = out.onUsageError
	app.Commands = initializeCLICommands(func() ctr.Client { return runtimeClient }, out.rec)
	setUsageErrorHandler(app.Commands, out.onUsageError)
	app.Flags = globalFlags(rootCertPath)
	return app
}

func before(c *cli.Context, out *eventOutput) error {
	if err := out.configure(c); err != nil {
		return err
	}
	if c.GlobalInt("max-targets") < 0 {
		return chaos.ErrNegativeMaxTargets
	}
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
