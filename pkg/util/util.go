package util

import (
	"errors"
	"time"
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
	} else if i, err := time.ParseDuration(interval); err == nil {
		return i, nil
	} else {
		return 0, err
	}
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
			return 0, err
		}
	}
	if interval != 0 && duration >= interval {
		return 0, errors.New("duration must be shorter than interval")
	}
	return duration, nil
}
