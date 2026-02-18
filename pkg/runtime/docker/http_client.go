package docker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

const defaultTimeout = 30 * time.Second

// HTTPClient create new http client to connect to the docker daemon
func HTTPClient(daemonURL string, tlsConfig *tls.Config) (*http.Client, error) {
	u, err := url.Parse(daemonURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse docker daemon url: %w", err)
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
		httpTransport.DialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, timeout) //nolint:wrapcheck // we can't wrap this error
		}
	case "unix":
		socketPath := address.Path
		unixDial := func(_ context.Context, _ string, _ string) (net.Conn, error) {
			return net.DialTimeout("unix", socketPath, timeout) //nolint:wrapcheck // we can't wrap this error
		}
		httpTransport.DialContext = unixDial
		// Override the main URL object so the HTTP lib won't complain
		address.Scheme = "http"
		address.Host = "unix.sock"
		address.Path = ""
	}
	return &http.Client{Transport: httpTransport}, nil
}
