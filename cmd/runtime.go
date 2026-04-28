package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/containerd"
	"github.com/alexei-led/pumba/pkg/runtime/docker"
	"github.com/alexei-led/pumba/pkg/runtime/podman"
	"github.com/urfave/cli"
)

// Runtime client factories. Package-level vars so tests can swap them without
// requiring a real Docker/containerd/podman socket.
var (
	newDockerClient     = docker.NewClient
	newContainerdClient = containerd.NewClient
	newPodmanClient     = podman.NewClient
)

// createRuntimeClient constructs the container.Client for the runtime selected
// via --runtime. Extracted from before() to keep gocyclo under the 15 limit
// and to give unit tests a single function to exercise.
func createRuntimeClient(c *cli.Context) (ctr.Client, error) {
	f := cliflags.NewV1FromApp(c)
	switch runtime := f.String("runtime"); runtime {
	case "docker":
		// tlsConfig still reads *cli.Context directly: it mixes flag reads with
		// os.ReadFile/x509 helpers, so threading the adapter would add noise
		// without payoff for the v3 migration this abstraction targets.
		tlsCfg, err := tlsConfig(c)
		if err != nil {
			return nil, err
		}
		client, err := newDockerClient(f.String("host"), tlsCfg)
		if err != nil {
			return nil, fmt.Errorf("could not create Docker client: %w", err)
		}
		return client, nil
	case "containerd":
		client, err := newContainerdClient(f.String("containerd-socket"), f.String("containerd-namespace"))
		if err != nil {
			return nil, fmt.Errorf("could not create containerd client: %w", err)
		}
		return client, nil
	case "podman":
		client, err := newPodmanClient(f.String("podman-socket"))
		if err != nil {
			return nil, fmt.Errorf("could not create podman client: %w", err)
		}
		return client, nil
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", runtime)
	}
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
