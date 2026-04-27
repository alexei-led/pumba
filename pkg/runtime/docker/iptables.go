package docker

import (
	"context"
	"fmt"
	"net"
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
		return client.ipTablesContainer(ctx, req.Container, req.CmdPrefix, req.CmdSuffix, req.Sidecar.Image, req.Sidecar.Pull, req.DryRun)
	}
	return client.ipTablesContainerWithIPFilter(ctx, req.Container, req.CmdPrefix, req.CmdSuffix, req.SrcIPs, req.DstIPs, req.SPorts, req.DPorts, req.Sidecar.Image, req.Sidecar.Pull, req.DryRun)
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
		return client.ipTablesContainer(ctx, req.Container, req.CmdPrefix, req.CmdSuffix, req.Sidecar.Image, req.Sidecar.Pull, req.DryRun)
	}
	return client.ipTablesContainerWithIPFilter(ctx, req.Container, req.CmdPrefix, req.CmdSuffix, req.SrcIPs, req.DstIPs, req.SPorts, req.DPorts, req.Sidecar.Image, req.Sidecar.Pull, req.DryRun)
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
	return client.runSidecar(ctx, c, argsList, tcimg, "iptables", pull)
}
