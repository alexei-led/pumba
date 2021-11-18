package container

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
)

const defaultTimeout = 30 * time.Second

// HTTPClient create new http client to connect to the docker daemon
func HTTPClient(daemonURL string, tlsConfig *tls.Config) (*http.Client, error) {
	u, err := url.Parse(daemonURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse docker daemon url")
	}
	if u.Scheme == "" || u.Scheme == "tcp" {
		if tlsConfig == nil {
			//nolint:goconst
			u.Scheme = "http"
		} else {
			u.Scheme = "https"
		}
	}

	return newHTTPClient(u, tlsConfig, defaultTimeout)
}

func newHTTPClient(address *url.URL, tlsConfig *tls.Config, timeout time.Duration) (*http.Client, error) {
	httpTransport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	switch address.Scheme {
	default:
		httpTransport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, timeout) //nolint:wrapcheck
		}
	case "unix":
		socketPath := address.Path
		unixDial := func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.DialTimeout("unix", socketPath, timeout) //nolint:wrapcheck
		}
		httpTransport.DialContext = unixDial
		// Override the main URL object so the HTTP lib won't complain
		address.Scheme = "http"
		address.Host = "unix.sock"
		address.Path = ""
	}
	return &http.Client{Transport: httpTransport}, nil
}
