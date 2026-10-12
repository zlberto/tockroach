package tockroach

import (
	"math"
	"time"
)

func systemTime() (time.Time, time.Duration, error) {
	return sampleTime(elapsedTime, time.Now)
}

func sampleTime(elapsed func() (time.Duration, error), wall func() time.Time) (time.Time, time.Duration, error) {
	// Pair wall time with an earlier elapsed reading so later samples retain
	// any pause between these reads. A pause after the elapsed read can still
	// make this particular result stale until the next sample.
	e, err := elapsed()
	if err != nil {
		return time.Time{}, 0, ErrUnavailable
	}
	return wall().UTC(), e, nil
}

func elapsedDuration(sec, nsec int64) (time.Duration, error) {
	const billion = int64(time.Second)
	if sec < 0 || nsec < 0 || nsec >= billion || sec > (math.MaxInt64-nsec)/billion {
		return 0, ErrUnavailable
	}
	return time.Duration(sec)*time.Second + time.Duration(nsec), nil
}
