package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

// IPTablesContainer injects sidecar iptables container into the given container network namespace
func (client dockerClient) IPTablesContainer(ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, duration time.Duration, img string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":          c.Name(),
		"id":            c.ID(),
		"commandPrefix": cmdPrefix,
		"commandSuffix": cmdSuffix,
		"srcIPs":        srcIPs,
		"dstIPs":        dstIPs,
		"sports":        sports,
		"dports":        dports,
		"duration":      duration,
		"img":           img,
		"pull":          pull,
		"dryrun":        dryrun,
	}).Info("running iptables on container")
	if len(srcIPs) == 0 && len(dstIPs) == 0 && len(sports) == 0 && len(dports) == 0 {
		return client.ipTablesContainer(ctx, c, cmdPrefix, cmdSuffix, img, pull, dryrun)
	}
	return client.ipTablesContainerWithIPFilter(ctx, c, cmdPrefix, cmdSuffix, srcIPs, dstIPs, sports, dports, img, pull, dryrun)
}

// StopIPTablesContainer stops the iptables container injected into the given container network namespace
func (client dockerClient) StopIPTablesContainer(ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, img string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":          c.Name(),
		"id":            c.ID(),
		"commandPrefix": cmdPrefix,
		"commandSuffix": cmdSuffix,
		"srcIPs":        srcIPs,
		"dstIPs":        dstIPs,
		"sports":        sports,
		"dports":        dports,
		"img":           img,
		"pull":          pull,
		"dryrun":        dryrun,
	}).Info("stopping netem on container")
	if len(srcIPs) == 0 && len(dstIPs) == 0 && len(sports) == 0 && len(dports) == 0 {
		return client.ipTablesContainer(ctx, c, cmdPrefix, cmdSuffix, img, pull, dryrun)
	}
	return client.ipTablesContainerWithIPFilter(ctx, c, cmdPrefix, cmdSuffix, srcIPs, dstIPs, sports, dports, img, pull, dryrun)
}

func (client dockerClient) ipTablesContainer(ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, img string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":      c.Name(),
		"id":        c.ID(),
		"cmdPrefix": strings.Join(cmdPrefix, " "),
		"cmdSuffix": strings.Join(cmdSuffix, " "),
		"img":       img,
		"pull":      pull,
		"dryrun":    dryrun,
	}).Debug("execute iptables for container")
	if !dryrun {
		var command []string
		command = append(command, cmdPrefix...)
		command = append(command, cmdSuffix...)
		log.WithField("iptables", strings.Join(command, " ")).Debug("executing iptables")
		return client.ipTablesCommands(ctx, c, [][]string{command}, img, pull)
	}
	return nil
}

func (client dockerClient) ipTablesContainerWithIPFilter(ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string,
	srcIPs, dstIPs []*net.IPNet, sports, dports []string, img string, pull bool, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"srcIPs": srcIPs,
		"dstIPs": dstIPs,
		"Sports": sports,
		"Dports": dports,
		"img":    img,
		"pull":   pull,
		"dryrun": dryrun,
	}).Debug("execute iptables for container with IP(s) filter")
	if !dryrun {
		// use docker client ExecStart to run iptables rules to filter network
		commands := [][]string{}

		// See more about the iptables statistics extension: https://www.man7.org/linux/man-pages/man8/iptables-extensions.8.html
		// # drop traffic to a specific source address

		for _, ip := range srcIPs {
			cmd := []string{}
			cmd = append(cmd, cmdPrefix...)
			cmd = append(cmd, "-s", ip.String())
			cmd = append(cmd, cmdSuffix...)
			commands = append(commands, cmd)
		}

		// # drop traffic to a specific destination address
		for _, ip := range dstIPs {
			cmd := []string{}
			cmd = append(cmd, cmdPrefix...)
			cmd = append(cmd, "-d", ip.String())
			cmd = append(cmd, cmdSuffix...)
			commands = append(commands, cmd)
		}

		// # drop traffic to a specific source port
		for _, sport := range sports {
			cmd := []string{}
			cmd = append(cmd, cmdPrefix...)
			cmd = append(cmd, "--sport", sport)
			cmd = append(cmd, cmdSuffix...)
			commands = append(commands, cmd)
		}

		// # drop traffic to a specific destination port
		for _, dport := range dports {
			cmd := []string{}
			cmd = append(cmd, cmdPrefix...)
			cmd = append(cmd, "--dport", dport)
			cmd = append(cmd, cmdSuffix...)
			commands = append(commands, cmd)
		}

		err := client.ipTablesCommands(ctx, c, commands, img, pull)
		if err != nil {
			return fmt.Errorf("failed to run iptables commands: %w", err)
		}
	}
	return nil
}

func (client dockerClient) ipTablesCommands(ctx context.Context, c *ctr.Container, argsList [][]string, tcimg string, pull bool) error {
	if tcimg == "" {
		for _, args := range argsList {
			if err := client.execOnContainer(ctx, c, "iptables", args, true); err != nil {
				return fmt.Errorf("error running iptables command on container: %v: %w", strings.Join(args, " "), err)
			}
		}
		return nil
	}
	return client.ipTablesContainerCommands(ctx, c, argsList, tcimg, pull)
}

// execute iptables commands using other container (with iproute2 and bind-tools package installed), using target container network stack
// try to use `biarca/iptables` img (Alpine + iproute2 and bind-tools package)
//
//nolint:dupl // intentionally parallel to tcContainerCommands; keeping them separate reads clearer at callsite
func (client dockerClient) ipTablesContainerCommands(ctx context.Context, target *ctr.Container, argsList [][]string, img string, pull bool) error {
	log.WithFields(log.Fields{
		"container": target.ID(),
		"img":       img,
		"pull":      pull,
		"args-list": argsList,
	}).Debug("executing iptables command in a separate container joining target container network namespace")

	// host config
	hconfig := ctypes.HostConfig{
		// Don't auto-remove, since we may want to run multiple commands
		AutoRemove: false,
		// NET_ADMIN is required for "tc netem"
		CapAdd: []string{"NET_ADMIN"},
		// use target container network stack
		NetworkMode: ctypes.NetworkMode("container:" + target.ID()),
		// others
		PortBindings: nat.PortMap{},
		DNS:          []string{},
		DNSOptions:   []string{},
		DNSSearch:    []string{},
	}
	log.WithField("network", hconfig.NetworkMode).Debug("network mode")
	// pull docker img if required: can pull only public imgs
	if pull {
		log.WithField("img", img).Debug("pulling iptables-img")
		events, err := client.imageAPI.ImagePull(ctx, img, imagetypes.PullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull iptables-img: %w", err)
		}
		defer events.Close()
		d := json.NewDecoder(events)
		var pullResponse *imagePullResponse
		for {
			if err = d.Decode(&pullResponse); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return fmt.Errorf("failed to decode docker pull response for iptables-img: %w", err)
			}
			log.Debug(pullResponse)
		}
	}

	// container config, keep the container alive by tailing /dev/null.
	// StopSignal: SIGKILL avoids the 10 s SIGTERM grace period on `rm -f`
	// (tail as PID 1 ignores SIGTERM), matching tcContainerCommands.
	config := ctypes.Config{
		Labels:     map[string]string{"com.gaiaadm.pumba.skip": "true"},
		Entrypoint: []string{"tail"},
		Cmd:        []string{"-f", "/dev/null"},
		Image:      img,
		StopSignal: "SIGKILL",
	}

	createResponse, err := client.containerAPI.ContainerCreate(ctx, &config, &hconfig, nil, nil, "")

	log.WithField("img", config.Image).Debug("creating iptables-container")
	if err != nil {
		return fmt.Errorf("failed to create iptables-container from iptables-img: %w", err)
	}
	log.WithField("id", createResponse.ID).Debug("iptables container created, starting it")
	err = client.containerAPI.ContainerStart(ctx, createResponse.ID, ctypes.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start iptables-container: %w", err)
	}

	for _, args := range argsList {
		if err = client.ipTablesExecCommand(ctx, createResponse.ID, args); err != nil {
			_ = client.removeSidecar(ctx, createResponse.ID)
			return fmt.Errorf("error running iptables command on container: %v: %w", strings.Join(args, " "), err)
		}
	}

	if err = client.removeSidecar(ctx, createResponse.ID); err != nil {
		return fmt.Errorf("failed to remove iptables-container: %w", err)
	}

	return nil
}
