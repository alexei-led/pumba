package podman

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/docker"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
)

// apiBackend is the narrow Docker SDK surface the podmanClient exercises for
// paths it owns (bootstrap /info; stress-ng sidecar). Interface-typed so
// tests can inject a fake without standing up an HTTP transport. Both
// *dockerapi.Client (production) and *mocks.APIClient (tests) satisfy it.
type apiBackend interface {
	Info(ctx context.Context) (system.Info, error)
	ContainerInspect(ctx context.Context, containerID string) (ctypes.InspectResponse, error)
	ImagePull(ctx context.Context, refStr string, options imagetypes.PullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *ctypes.Config, hostConfig *ctypes.HostConfig, networkingConfig *networktypes.NetworkingConfig, platform *v1.Platform, containerName string) (ctypes.CreateResponse, error)
	ContainerAttach(ctx context.Context, container string, options ctypes.AttachOptions) (types.HijackedResponse, error)
	ContainerStart(ctx context.Context, container string, options ctypes.StartOptions) error
	Close() error
}

// bootstrapTimeout bounds the initial /info probe. The resolveSocket step only
// stat()s unix paths, so a file that exists but doesn't speak the API will hang
// here until this timeout expires — acceptable for a one-shot startup check.
const bootstrapTimeout = 10 * time.Second

// Test seams for NewClient. Tests swap these via helpers to avoid real network
// activity while exercising the bootstrap flow.
var (
	newAPIClient = docker.NewAPIClient
	newDelegate  = docker.NewFromAPI
	fetchInfo    = func(ctx context.Context, api apiBackend) (system.Info, error) {
		return api.Info(ctx)
	}
)

// podmanClient implements ctr.Client for the Podman runtime via its
// Docker-compatible API socket. The embedded ctr.Client handles every method
// where Docker and Podman agree; this struct overrides only the ones that
// differ: chaos commands that require kernel privileges (guarded against
// rootless sockets) and — in Task 6 — StressContainer (cgroup leaf naming
// divergence).
type podmanClient struct {
	ctr.Client
	api       apiBackend
	rootless  bool
	socketURI string
}

// NewClient connects to Podman's Docker-compatible API socket and returns a
// ready-to-use ctr.Client. When explicitSocket is empty the socket is
// auto-detected (see resolveSocket); when set, it wins outright — no fallback.
// A single /info call at bootstrap records whether the socket is rootless so
// subsequent chaos commands that need kernel privileges can fail fast with a
// useful diagnostic instead of a cryptic error from inside the sidecar.
func NewClient(explicitSocket string) (ctr.Client, error) {
	uri, source, err := resolveSocket(explicitSocket)
	if err != nil {
		return nil, err
	}
	log.WithFields(log.Fields{"socket": uri, "source": source}).Debug("resolved podman socket")

	api, err := newAPIClient(uri, nil)
	if err != nil {
		return nil, fmt.Errorf("podman runtime: create api client for %s: %w", uri, err)
	}

	delegate, err := newDelegate(api)
	if err != nil {
		_ = api.Close()
		return nil, fmt.Errorf("podman runtime: wrap docker delegate: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), bootstrapTimeout)
	defer cancel()
	info, err := fetchInfo(ctx, api)
	if err != nil {
		_ = api.Close()
		return nil, fmt.Errorf("podman runtime: query /info on %s: %w", uri, err)
	}

	rootless := detectRootless(&info)
	log.WithFields(log.Fields{"socket": uri, "rootless": rootless}).Debug("podman client ready")

	return &podmanClient{
		Client:    delegate,
		api:       api,
		rootless:  rootless,
		socketURI: uri,
	}, nil
}

// Close releases the underlying Docker SDK client's HTTP transport. Shadows
// the embedded delegate's no-op Close so the connection cache doesn't leak.
func (p *podmanClient) Close() error {
	if p.api == nil {
		return nil
	}
	return p.api.Close()
}

// NetemContainer injects a netem qdisc into the target's network namespace.
// Rootless Podman cannot grant NET_ADMIN to a sidecar in the target's netns;
// fail fast with a clear diagnostic rather than an opaque sidecar error.
func (p *podmanClient) NetemContainer(ctx context.Context, c *ctr.Container, netInterface string, netemCmd []string, ips []*net.IPNet, sports, dports []string, duration time.Duration, tcimg string, pull, dryrun bool) error {
	if p.rootless {
		return rootlessError("netem", p.socketURI)
	}
	return p.Client.NetemContainer(ctx, c, netInterface, netemCmd, ips, sports, dports, duration, tcimg, pull, dryrun)
}

// StopNetemContainer removes the netem rules installed by NetemContainer.
// Mirrors the rootless guard so stop-without-start on a rootless socket also
// returns the same diagnostic instead of a cryptic sidecar failure.
func (p *podmanClient) StopNetemContainer(ctx context.Context, c *ctr.Container, netInterface string, ip []*net.IPNet, sports, dports []string, tcimg string, pull, dryrun bool) error {
	if p.rootless {
		return rootlessError("netem", p.socketURI)
	}
	return p.Client.StopNetemContainer(ctx, c, netInterface, ip, sports, dports, tcimg, pull, dryrun)
}

// IPTablesContainer installs iptables rules in the target's network namespace.
// Same rootless constraint as NetemContainer.
func (p *podmanClient) IPTablesContainer(ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, duration time.Duration, img string, pull, dryrun bool) error {
	if p.rootless {
		return rootlessError("iptables", p.socketURI)
	}
	return p.Client.IPTablesContainer(ctx, c, cmdPrefix, cmdSuffix, srcIPs, dstIPs, sports, dports, duration, img, pull, dryrun)
}

// StopIPTablesContainer removes the iptables rules installed by
// IPTablesContainer. Mirrors the rootless guard.
func (p *podmanClient) StopIPTablesContainer(ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, img string, pull, dryrun bool) error {
	if p.rootless {
		return rootlessError("iptables", p.socketURI)
	}
	return p.Client.StopIPTablesContainer(ctx, c, cmdPrefix, cmdSuffix, srcIPs, dstIPs, sports, dports, img, pull, dryrun)
}
