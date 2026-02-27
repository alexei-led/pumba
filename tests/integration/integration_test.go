//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

var (
	pumba       string
	dockerCli   *client.Client
	nettoolsImg string
)

const (
	defaultImage  = "alpine:latest"
	netshootImage = "nicolaka/netshoot:latest"
)

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	// 1. Find or build pumba binary
	pumba = findPumba()
	if pumba == "" {
		fmt.Fprintln(os.Stderr, "FATAL: pumba binary not found; run 'make build' first")
		return 1
	}
	fmt.Fprintf(os.Stderr, "Using pumba binary: %s\n", pumba)

	// 2. Create shared Docker client
	var err error
	dockerCli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: cannot create Docker client: %v\n", err)
		return 1
	}
	defer dockerCli.Close()

	// 3. Pull required images
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pullImages(ctx)

	// 4. Detect nettools image
	nettoolsImg = detectNettoolsImage(ctx)
	fmt.Fprintf(os.Stderr, "Using nettools image: %s\n", nettoolsImg)

	// 5. Cleanup leaked test containers/sidecars
	defer cleanupLeaked()

	// 6. Run tests
	return m.Run()
}

func findPumba() string {
	// Check common locations
	candidates := []string{
		"/usr/local/bin/pumba",
		".bin/github.com/alexei-led/pumba",
		"../../.bin/github.com/alexei-led/pumba",
	}
	// Check PATH first
	if p, err := exec.LookPath("pumba"); err == nil {
		return p
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func pullImages(ctx context.Context) {
	images := []string{defaultImage, netshootImage}
	for _, img := range images {
		// Check if already present
		_, _, err := dockerCli.ImageInspectWithRaw(ctx, img)
		if err == nil {
			continue
		}
		fmt.Fprintf(os.Stderr, "Pulling %s...\n", img)
		rc, err := dockerCli.ImagePull(ctx, "docker.io/"+img, image.PullOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: failed to pull %s: %v\n", img, err)
			continue
		}
		_, _ = io.Copy(io.Discard, rc)
		rc.Close()
	}
}

func detectNettoolsImage(ctx context.Context) string {
	// Prefer local image if in CI
	if os.Getenv("CI") == "true" {
		return "pumba-alpine-nettools:local"
	}
	img := "ghcr.io/alexei-led/pumba-alpine-nettools:latest"
	if _, _, err := dockerCli.ImageInspectWithRaw(ctx, img); err == nil {
		return img
	}
	// Try pulling
	rc, err := dockerCli.ImagePull(ctx, img, image.PullOptions{})
	if err == nil {
		_, _ = io.Copy(io.Discard, rc)
		rc.Close()
		return img
	}
	// Fall back to local
	return "pumba-alpine-nettools:local"
}

func cleanupLeaked() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Remove containers with our test prefix
	containers, err := dockerCli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("name", "inttest-")),
	})
	if err != nil {
		return
	}
	for _, c := range containers {
		_ = dockerCli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
	}

	// Remove sidecar containers (pumba.skip label)
	sidecars, err := dockerCli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", "com.gaiaadm.pumba.skip=true")),
	})
	if err != nil {
		return
	}
	for _, c := range sidecars {
		for _, name := range c.Names {
			if strings.Contains(name, "inttest-") {
				_ = dockerCli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
				break
			}
		}
	}
}
