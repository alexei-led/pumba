package util

import (
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"
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

// GetIntervalValue get interval value from string duration
func GetIntervalValue(interval string) (time.Duration, error) {
	// get recurrent time interval
	if interval == "" {
		return 0, nil
	}
	i, err := time.ParseDuration(interval)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse interval")
	}
	return i, nil
}

// GetDurationValue get duration and make sure it's smaller than interval
func GetDurationValue(durationStr string, interval time.Duration) (time.Duration, error) {
	var err error
	var duration time.Duration
	if durationStr == "" {
		return 0, errors.New("undefined duration")
	}
	if durationStr != "" {
		duration, err = time.ParseDuration(durationStr)
		if err != nil {
			return 0, errors.Wrap(err, "failed to parse duration")
		}
	}
	if interval != 0 && duration >= interval {
		return 0, errors.New("duration must be shorter than interval")
	}
	return duration, nil
}

// CIDRNotation Ensure IP string is in CIDR notation
func CIDRNotation(ip string) string {
	if !strings.Contains(ip, "/") {
		return ip + "/32"
	}
	return ip
}

// ParseCIDR Parse IP string to IPNet
func ParseCIDR(ip string) (*net.IPNet, error) {
	cidr := CIDRNotation(ip)
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	return ipNet, nil
}
