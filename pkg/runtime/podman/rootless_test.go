package podman

import (
	"testing"

	"github.com/docker/docker/api/types/system"
	"github.com/stretchr/testify/assert"
)

func TestDetectRootless(t *testing.T) {
	tests := []struct {
		name string
		info *system.Info
		want bool
	}{
		{
			name: "rootless marker present",
			info: &system.Info{SecurityOptions: []string{"name=rootless"}},
			want: true,
		},
		{
			name: "rootless marker with extra options",
			info: &system.Info{SecurityOptions: []string{
				"name=seccomp,profile=default",
				"name=rootless",
				"name=cgroupns",
			}},
			want: true,
		},
		{
			name: "rootless as substring inside larger opt",
			info: &system.Info{SecurityOptions: []string{"name=rootless,foo=bar"}},
			want: true,
		},
		{
			name: "no rootless marker",
			info: &system.Info{SecurityOptions: []string{
				"name=seccomp,profile=default",
				"name=cgroupns",
			}},
			want: false,
		},
		{
			name: "empty security options",
			info: &system.Info{SecurityOptions: []string{}},
			want: false,
		},
		{
			name: "empty info",
			info: &system.Info{},
			want: false,
		},
		{
			name: "nil info",
			info: nil,
			want: false,
		},
		{
			name: "substring match only — still contains rootless token",
			info: &system.Info{SecurityOptions: []string{"name=rootlesskit"}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectRootless(tt.info))
		})
	}
}

func TestRootlessError(t *testing.T) {
	err := rootlessError("netem", "unix:///run/user/1000/podman/podman.sock")
	require := assert.New(t)

	require.Error(err)
	msg := err.Error()
	require.Contains(msg, "netem")
	require.Contains(msg, "unix:///run/user/1000/podman/podman.sock")
	require.Contains(msg, "podman machine set --rootful")
	require.Contains(msg, "/run/podman/podman.sock")
	require.Contains(msg, "rootful")
}
