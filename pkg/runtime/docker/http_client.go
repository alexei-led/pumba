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

	dialer := &net.Dialer{Timeout: timeout}
	switch address.Scheme {
	default:
		httpTransport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr) //nolint:wrapcheck // pass dialer error as-is
		}
	case "unix":
		socketPath := address.Path
		httpTransport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath) //nolint:wrapcheck // pass dialer error as-is
		}
		// Override the main URL object so the HTTP lib won't complain
		address.Scheme = "http"
		address.Host = "unix.sock"
		address.Path = ""
	}
	return &http.Client{Transport: httpTransport}, nil
}
