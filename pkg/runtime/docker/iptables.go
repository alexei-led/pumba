package docker

import (
	"context"
	"fmt"
	"strings"

	ctr "github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// IPTablesContainer injects sidecar iptables container into the given container network namespace
func (client dockerClient) IPTablesContainer(ctx context.Context, req *ctr.IPTablesRequest) error {
	log.WithFields(log.Fields{
		"name":          req.Container.Name(),
		"id":            req.Container.ID(),
		"commandPrefix": req.CmdPrefix,
		"commandSuffix": req.CmdSuffix,
		"srcIPs":        req.SrcIPs,
		"dstIPs":        req.DstIPs,
		"sports":        req.SPorts,
		"dports":        req.DPorts,
		"duration":      req.Duration,
		"img":           req.Sidecar.Image,
		"pull":          req.Sidecar.Pull,
		"dryrun":        req.DryRun,
	}).Info("running iptables on container")
	if len(req.SrcIPs) == 0 && len(req.DstIPs) == 0 && len(req.SPorts) == 0 && len(req.DPorts) == 0 {
		return client.ipTablesContainer(ctx, req)
	}
	return client.ipTablesContainerWithIPFilter(ctx, req)
}

// StopIPTablesContainer stops the iptables container injected into the given container network namespace
func (client dockerClient) StopIPTablesContainer(ctx context.Context, req *ctr.IPTablesRequest) error {
	log.WithFields(log.Fields{
		"name":          req.Container.Name(),
		"id":            req.Container.ID(),
		"commandPrefix": req.CmdPrefix,
		"commandSuffix": req.CmdSuffix,
		"srcIPs":        req.SrcIPs,
		"dstIPs":        req.DstIPs,
		"sports":        req.SPorts,
		"dports":        req.DPorts,
		"img":           req.Sidecar.Image,
		"pull":          req.Sidecar.Pull,
		"dryrun":        req.DryRun,
	}).Info("stopping iptables on container")
	if len(req.SrcIPs) == 0 && len(req.DstIPs) == 0 && len(req.SPorts) == 0 && len(req.DPorts) == 0 {
		return client.ipTablesContainer(ctx, req)
	}
	return client.ipTablesContainerWithIPFilter(ctx, req)
}

func (client dockerClient) ipTablesContainer(ctx context.Context, req *ctr.IPTablesRequest) error {
	log.WithFields(log.Fields{
		"name":      req.Container.Name(),
		"id":        req.Container.ID(),
		"cmdPrefix": strings.Join(req.CmdPrefix, " "),
		"cmdSuffix": strings.Join(req.CmdSuffix, " "),
		"img":       req.Sidecar.Image,
		"pull":      req.Sidecar.Pull,
		"dryrun":    req.DryRun,
	}).Debug("execute iptables for container")
	if !req.DryRun {
		var command []string
		command = append(command, req.CmdPrefix...)
		command = append(command, req.CmdSuffix...)
		log.WithField("iptables", strings.Join(command, " ")).Debug("executing iptables")
		return client.ipTablesCommands(ctx, req.Container, [][]string{command}, req.Sidecar.Image, req.Sidecar.Pull)
	}
	return nil
}

func (client dockerClient) ipTablesContainerWithIPFilter(ctx context.Context, req *ctr.IPTablesRequest) error {
	log.WithFields(log.Fields{
		"name":   req.Container.Name(),
		"id":     req.Container.ID(),
		"srcIPs": req.SrcIPs,
		"dstIPs": req.DstIPs,
		"Sports": req.SPorts,
		"Dports": req.DPorts,
		"img":    req.Sidecar.Image,
		"pull":   req.Sidecar.Pull,
		"dryrun": req.DryRun,
	}).Debug("execute iptables for container with IP(s) filter")
	if !req.DryRun {
		// use docker client ExecStart to run iptables rules to filter network
		commands := [][]string{}

		// See more about the iptables statistics extension: https://www.man7.org/linux/man-pages/man8/iptables-extensions.8.html
		// # drop traffic to a specific source address

		for _, ip := range req.SrcIPs {
			cmd := []string{}
			cmd = append(cmd, req.CmdPrefix...)
			cmd = append(cmd, "-s", ip.String())
			cmd = append(cmd, req.CmdSuffix...)
			commands = append(commands, cmd)
		}

		// # drop traffic to a specific destination address
		for _, ip := range req.DstIPs {
			cmd := []string{}
			cmd = append(cmd, req.CmdPrefix...)
			cmd = append(cmd, "-d", ip.String())
			cmd = append(cmd, req.CmdSuffix...)
			commands = append(commands, cmd)
		}

		// # drop traffic to a specific source port
		for _, sport := range req.SPorts {
			cmd := []string{}
			cmd = append(cmd, req.CmdPrefix...)
			cmd = append(cmd, "--sport", sport)
			cmd = append(cmd, req.CmdSuffix...)
			commands = append(commands, cmd)
		}

		// # drop traffic to a specific destination port
		for _, dport := range req.DPorts {
			cmd := []string{}
			cmd = append(cmd, req.CmdPrefix...)
			cmd = append(cmd, "--dport", dport)
			cmd = append(cmd, req.CmdSuffix...)
			commands = append(commands, cmd)
		}

		err := client.ipTablesCommands(ctx, req.Container, commands, req.Sidecar.Image, req.Sidecar.Pull)
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
	return client.runSidecar(ctx, c, argsList, tcimg, "iptables", pull)
}
