package docker

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHTTPClient(t *testing.T) {
	tests := []struct {
		name      string
		daemonURL string
		tlsConfig *tls.Config
		wantErr   bool
	}{
		{
			name:      "tcp url with no TLS",
			daemonURL: "tcp://localhost:2375",
			tlsConfig: nil,
			wantErr:   false,
		},
		{
			name:      "http url with no TLS",
			daemonURL: "http://localhost:2375",
			tlsConfig: nil,
			wantErr:   false,
		},
		{
			name:      "tcp url with TLS",
			daemonURL: "tcp://localhost:2376",
			tlsConfig: &tls.Config{},
			wantErr:   false,
		},
		{
			name:      "https url with TLS",
			daemonURL: "https://localhost:2376",
			tlsConfig: &tls.Config{},
			wantErr:   false,
		},
		{
			name:      "unix socket",
			daemonURL: "unix:///var/run/docker.sock",
			tlsConfig: nil,
			wantErr:   false,
		},
		{
			name:      "invalid URL",
			daemonURL: "://invalid-url",
			tlsConfig: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := HTTPClient(tt.daemonURL, tt.tlsConfig)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				transport, ok := client.Transport.(*http.Transport)
				assert.True(t, ok)

				if tt.tlsConfig != nil {
					assert.Equal(t, tt.tlsConfig, transport.TLSClientConfig)
				}

				if tt.daemonURL != "" && strings.HasPrefix(tt.daemonURL, "unix:") {
					assert.NotNil(t, transport.DialContext)
				}
			}
		})
	}
}

func TestNewHTTPClient(t *testing.T) {
	tests := []struct {
		name    string
		address *url.URL
		tlsConf *tls.Config
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "http scheme",
			address: &url.URL{Scheme: "http", Host: "localhost:2375"},
			tlsConf: nil,
			timeout: 10 * time.Second,
			wantErr: false,
		},
		{
			name:    "https scheme with TLS",
			address: &url.URL{Scheme: "https", Host: "localhost:2376"},
			tlsConf: &tls.Config{InsecureSkipVerify: true},
			timeout: 10 * time.Second,
			wantErr: false,
		},
		{
			name:    "unix scheme",
			address: &url.URL{Scheme: "unix", Path: "/var/run/docker.sock"},
			tlsConf: nil,
			timeout: 10 * time.Second,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := newHTTPClient(tt.address, tt.tlsConf, tt.timeout)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				transport, ok := client.Transport.(*http.Transport)
				assert.True(t, ok)

				if tt.tlsConf != nil {
					assert.Equal(t, tt.tlsConf, transport.TLSClientConfig)
				}

				assert.NotNil(t, transport.DialContext)

				if tt.address.Scheme == "unix" {
					assert.Equal(t, "http", tt.address.Scheme)
					assert.Equal(t, "unix.sock", tt.address.Host)
					assert.Equal(t, "", tt.address.Path)
				}
			}
		})
	}
}
