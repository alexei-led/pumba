package util

import (
	"errors"
	"time"
	"strings"
	"net"

	log "github.com/sirupsen/logrus"
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
		log.Debug("no interval specified, running only once")
		return 0, nil
	} else if i, err := time.ParseDuration(interval); err == nil {
		log.WithField("interval", interval).Debug("setting command interval")
		return i, nil
	} else {
		log.WithError(err).WithField("interval", interval).Error("failed to parse interval")
		return 0, err
	}
}

// GetDurationValue get duration and make sure it's smaller than interval
func GetDurationValue(durationStr string, interval time.Duration) (time.Duration, error) {
	var err error
	var duration time.Duration
	if durationStr == "" {
		log.Error("undefined duration")
		return 0, errors.New("undefined duration")
	}
	if durationStr != "" {
		duration, err = time.ParseDuration(durationStr)
		if err != nil {
			log.WithError(err).WithField("duration", durationStr).Error("failed to parse duration")
			return 0, err
		}
	}
	if interval != 0 && duration >= interval {
		log.Error("duration must be shorter than interval")
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
func ParseCIDR(ip string) *net.IPNet {
	cidr := CIDRNotation(ip)

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		log.Error(err)
		return nil
	}
	return ipNet
}
