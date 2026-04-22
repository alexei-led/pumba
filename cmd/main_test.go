package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"os"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/urfave/cli"
)

type mainTestSuite struct {
	suite.Suite
}

func (s *mainTestSuite) SetupSuite() {
	topContext = context.TODO()
}

func (s *mainTestSuite) TearDownSuite() {
}

func (s *mainTestSuite) SetupTest() {
}

func (s *mainTestSuite) TearDownTest() {
}

func (s *mainTestSuite) Test_main() {
	os.Args = []string{"pumba", "-v"}
	main()
}

func (s *mainTestSuite) Test_handleSignals() {
	handleSignals()
}

func TestMainTestSuite(t *testing.T) {
	suite.Run(t, new(mainTestSuite))
}

// newRuntimeTestContext builds a cli.Context populated with the global flag
// defaults and overrides from values. Lets createRuntimeClient read flags
// without running the whole urfave/cli app lifecycle.
func newRuntimeTestContext(t *testing.T, values map[string]string) *cli.Context {
	t.Helper()
	app := cli.NewApp()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range globalFlags("/tmp/certs") {
		f.Apply(fs)
	}
	for k, v := range values {
		require.NoError(t, fs.Set(k, v))
	}
	return cli.NewContext(app, fs, nil)
}

// restoreFactories snapshots the runtime-client factory vars and returns a
// cleanup func so tests can swap them without leaking state across subtests.
func restoreFactories(t *testing.T) {
	t.Helper()
	origDocker := newDockerClient
	origContainerd := newContainerdClient
	origPodman := newPodmanClient
	t.Cleanup(func() {
		newDockerClient = origDocker
		newContainerdClient = origContainerd
		newPodmanClient = origPodman
	})
}

func TestCreateRuntimeClient_Podman(t *testing.T) {
	restoreFactories(t)

	var gotSocket string
	sentinel := &ctr.MockClient{}
	newPodmanClient = func(socket string) (ctr.Client, error) {
		gotSocket = socket
		return sentinel, nil
	}

	ctx := newRuntimeTestContext(t, map[string]string{
		"runtime":       "podman",
		"podman-socket": "unix:///tmp/podman.sock",
	})

	client, err := createRuntimeClient(ctx)
	require.NoError(t, err)
	assert.Same(t, sentinel, client)
	assert.Equal(t, "unix:///tmp/podman.sock", gotSocket)
}

func TestCreateRuntimeClient_PodmanEmptySocket(t *testing.T) {
	restoreFactories(t)

	var gotSocket string
	called := false
	newPodmanClient = func(socket string) (ctr.Client, error) {
		called = true
		gotSocket = socket
		return &ctr.MockClient{}, nil
	}

	ctx := newRuntimeTestContext(t, map[string]string{"runtime": "podman"})

	_, err := createRuntimeClient(ctx)
	require.NoError(t, err)
	assert.True(t, called, "podman factory should be invoked")
	assert.Empty(t, gotSocket, "empty --podman-socket should be passed through so the package auto-detects")
}

func TestCreateRuntimeClient_PodmanError(t *testing.T) {
	restoreFactories(t)

	newPodmanClient = func(string) (ctr.Client, error) {
		return nil, errors.New("boom")
	}

	ctx := newRuntimeTestContext(t, map[string]string{"runtime": "podman"})

	client, err := createRuntimeClient(ctx)
	assert.Nil(t, client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not create podman client")
	assert.Contains(t, err.Error(), "boom")
}

func TestCreateRuntimeClient_Docker(t *testing.T) {
	restoreFactories(t)

	var gotHost string
	sentinel := &ctr.MockClient{}
	newDockerClient = func(host string, _ *tls.Config) (ctr.Client, error) {
		gotHost = host
		return sentinel, nil
	}

	ctx := newRuntimeTestContext(t, map[string]string{
		"runtime": "docker",
		"host":    "unix:///var/run/docker.sock",
	})

	client, err := createRuntimeClient(ctx)
	require.NoError(t, err)
	assert.Same(t, sentinel, client)
	assert.Equal(t, "unix:///var/run/docker.sock", gotHost)
}

func TestCreateRuntimeClient_Containerd(t *testing.T) {
	restoreFactories(t)

	var gotSocket, gotNamespace string
	sentinel := &ctr.MockClient{}
	newContainerdClient = func(socket, namespace string) (ctr.Client, error) {
		gotSocket = socket
		gotNamespace = namespace
		return sentinel, nil
	}

	ctx := newRuntimeTestContext(t, map[string]string{
		"runtime":              "containerd",
		"containerd-socket":    "/tmp/ctr.sock",
		"containerd-namespace": "default",
	})

	client, err := createRuntimeClient(ctx)
	require.NoError(t, err)
	assert.Same(t, sentinel, client)
	assert.Equal(t, "/tmp/ctr.sock", gotSocket)
	assert.Equal(t, "default", gotNamespace)
}

func TestCreateRuntimeClient_Unknown(t *testing.T) {
	restoreFactories(t)

	ctx := newRuntimeTestContext(t, map[string]string{"runtime": "rkt"})

	client, err := createRuntimeClient(ctx)
	assert.Nil(t, client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported runtime: rkt")
}
