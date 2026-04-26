package cmd

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli"
)

func newTestCLIContext(args []string) *cli.Context {
	app := cli.NewApp()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = fs.Parse(args)
	return cli.NewContext(app, fs, nil)
}

func TestKillRequiresContainerArgument(t *testing.T) {
	cmd := &killContext{context: context.Background()}
	c := newTestCLIContext(nil)
	err := cmd.kill(c)
	assert.EqualError(t, err, "container name, list of names, or RE2 regex is required")
}

func TestStopRequiresContainerArgument(t *testing.T) {
	cmd := &stopContext{context: context.Background()}
	c := newTestCLIContext(nil)
	err := cmd.stop(c)
	assert.EqualError(t, err, "container name, list of names, or RE2 regex is required")
}

func TestRemoveRequiresContainerArgument(t *testing.T) {
	cmd := &removeContext{context: context.Background()}
	c := newTestCLIContext(nil)
	err := cmd.remove(c)
	assert.EqualError(t, err, "container name, list of names, or RE2 regex is required")
}
