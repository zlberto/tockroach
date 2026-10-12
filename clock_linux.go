//go:build linux && !android

package tockroach

import (
	"golang.org/x/sys/unix"
	"time"
)

func elapsedTime() (time.Duration, error) {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts); err != nil {
		return 0, ErrUnavailable
	}
	return elapsedDuration(int64(ts.Sec), int64(ts.Nsec))
}
