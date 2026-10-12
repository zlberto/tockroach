//go:build darwin && !ios

package tockroach

import (
	"golang.org/x/sys/unix"
	"time"
)

func elapsedTime() (time.Duration, error) {
	// Unlike CLOCK_UPTIME_RAW, CLOCK_MONOTONIC_RAW includes system sleep.
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &ts); err != nil {
		return 0, ErrUnavailable
	}
	return elapsedDuration(int64(ts.Sec), int64(ts.Nsec))
}
