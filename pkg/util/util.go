package util

import (
	"fmt"
	"github.com/pkg/errors"
	"net"
	"strconv"
	"strings"
)

// SliceContains checks if slice contains value
func SliceContains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}
	_, ok := set[item]
	return ok
}

// GetPorts will split the string of comma separated ports and return a list of ports
func GetPorts(ports string) ([]string, error) {
	portList := strings.Split(ports, ",")
	// Handle no port case
	if portList[0] == "" {
		return nil, nil
	}

	for _, port := range portList {
		err := verifyPort(port)

		if err != nil {
			return nil, errors.Wrap(err, "invalid port specified")
		}
	}

	return portList, nil
}

// verifyPort will make sure the port is numeric and within the correct range
func verifyPort(port string) error {
	if port == "" {
		return nil
	}
	portNum, err := strconv.ParseInt(port, 10, 64)
	if err != nil {
		return errors.Wrap(err, "failed to parse port as number")
	}
	if portNum < 0 || portNum > 65535 {
		return fmt.Errorf("Port is either below 0 or greater than 65535: " + port)
	}

	return nil
}

// ensure IP string is in CIDR notation
func cidrNotation(ip string) string {
	if !strings.Contains(ip, "/") {
		return ip + "/32"
	}
	return ip
}

// ParseCIDR Parse IP string to IPNet
func ParseCIDR(ip string) (*net.IPNet, error) {
	cidr := cidrNotation(ip)
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse CIDR")
	}
	return ipNet, nil
}
