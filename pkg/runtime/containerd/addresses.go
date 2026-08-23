package containerd

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"sort"
	"strings"

	ctr "github.com/alexei-led/pumba/pkg/container"
)

// ContainerAddresses reads addresses from the target task's network namespace.
func (c *containerdClient) ContainerAddresses(ctx context.Context, container *ctr.Container) ([]net.IP, error) {
	ctx = c.nsCtx(ctx)
	cntr, err := c.client.LoadContainer(ctx, container.ID())
	if err != nil {
		return nil, fmt.Errorf("failed to load container %s: %w", container.ID(), err)
	}
	task, err := cntr.Task(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get task for %s: %w", container.ID(), err)
	}
	pid := task.Pid()
	if pid == 0 {
		return nil, fmt.Errorf("container %s task has no PID", container.ID())
	}

	sameNamespace := c.sameNetworkNamespace
	if sameNamespace == nil {
		sameNamespace = isHostNetworkNamespace
	}
	hostNetwork, err := sameNamespace(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect network namespace for container %s: %w", container.ID(), err)
	}
	if hostNetwork {
		return nil, nil
	}

	readFile := c.readProcessFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	fibTrie, err := readFile(fmt.Sprintf("/proc/%d/net/fib_trie", pid))
	if err != nil {
		return nil, fmt.Errorf("failed to read IPv4 addresses for container %s: %w", container.ID(), err)
	}
	ipv6, err := readFile(fmt.Sprintf("/proc/%d/net/if_inet6", pid))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to read IPv6 addresses for container %s: %w", container.ID(), err)
	}
	return parseProcessAddresses(fibTrie, ipv6)
}

func isHostNetworkNamespace(pid uint32) (bool, error) {
	self, err := os.Stat("/proc/self/ns/net")
	if err != nil {
		return false, err
	}
	target, err := os.Stat(fmt.Sprintf("/proc/%d/ns/net", pid))
	if err != nil {
		return false, err
	}
	return os.SameFile(self, target), nil
}

func parseProcessAddresses(fibTrie, ifInet6 []byte) ([]net.IP, error) {
	ipv4, err := parseIPv4Addresses(fibTrie)
	if err != nil {
		return nil, err
	}
	ipv6, err := parseIPv6Addresses(ifInet6)
	if err != nil {
		return nil, err
	}
	addresses := make(map[string]net.IP, len(ipv4)+len(ipv6))
	for _, ip := range append(ipv4, ipv6...) {
		addresses[ip.String()] = ip
	}
	keys := make([]string, 0, len(addresses))
	for address := range addresses {
		keys = append(keys, address)
	}
	sort.Strings(keys)
	result := make([]net.IP, 0, len(keys))
	for _, address := range keys {
		result = append(result, addresses[address])
	}
	return result, nil
}

func parseIPv4Addresses(fibTrie []byte) ([]net.IP, error) {
	var addresses []net.IP
	var candidate net.IP
	scanner := bufio.NewScanner(strings.NewReader(string(fibTrie)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) == 2 && (fields[0] == "|--" || fields[0] == "+--") {
			candidate = net.ParseIP(fields[1]).To4()
			continue
		}
		if strings.Contains(line, "/32 host LOCAL") && isTargetAddress(candidate) {
			addresses = append(addresses, candidate)
		}
		candidate = nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse IPv4 addresses: %w", err)
	}
	return addresses, nil
}

func parseIPv6Addresses(ifInet6 []byte) ([]net.IP, error) {
	var addresses []net.IP
	scanner := bufio.NewScanner(strings.NewReader(string(ifInet6)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		value, err := hex.DecodeString(fields[0])
		if err != nil || len(value) != net.IPv6len {
			return nil, fmt.Errorf("invalid IPv6 address %q", fields[0])
		}
		ip := net.IP(value)
		if isTargetAddress(ip) {
			addresses = append(addresses, ip)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse IPv6 addresses: %w", err)
	}
	return addresses, nil
}

func isTargetAddress(ip net.IP) bool {
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsLinkLocalUnicast()
}
