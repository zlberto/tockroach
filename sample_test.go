package tockroach

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestPauseInEarlierSampleDoesNotExtendExpiry(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	elapsed := time.Hour
	// Whichever read happens first is followed by a simulated ten-minute pause.
	reads := 0
	readElapsed := func() (time.Duration, error) {
		reads++
		v := elapsed
		if reads == 1 {
			elapsed += 10 * time.Minute
		}
		return v, nil
	}
	readWall := func() time.Time {
		reads++
		v := base.Add(elapsed - time.Hour)
		if reads == 1 {
			elapsed += 10 * time.Minute
		}
		return v
	}
	c := NewWithSource(func() (time.Time, time.Duration, error) { return sampleTime(readElapsed, readWall) })
	if _, err := c.Now(); err != nil {
		t.Fatal(err)
	}
	// The next wall sample is rolled back. The elapsed floor must still account
	// for the pause that happened between the initial pair of system reads.
	base = base.Add(-time.Hour)
	got, err := c.Now()
	expires := time.Date(2026, 1, 1, 12, 5, 0, 0, time.UTC)
	if err != nil || got.Before(expires) {
		t.Fatalf("pause extended expiry: now=%v, expiry=%v, err=%v", got, expires, err)
	}
}

func TestElapsedDurationRejectsMalformedAndOverflow(t *testing.T) {
	for _, tc := range []struct {
		sec, nsec int64
		want      time.Duration
		invalid   bool
	}{
		{0, 0, 0, false},
		{1, 23, time.Second + 23, false},
		{9223372036, 854775807, time.Duration(math.MaxInt64), false},
		{-1, 0, 0, true},
		{0, -1, 0, true},
		{0, 1000000000, 0, true},
		{9223372036, 854775808, 0, true},
		{9223372037, 0, 0, true},
		{math.MaxInt64, 0, 0, true},
	} {
		got, err := elapsedDuration(tc.sec, tc.nsec)
		if tc.invalid {
			if got != 0 || !errors.Is(err, ErrUnavailable) {
				t.Errorf("accepted invalid elapsed (%d,%d): %v, %v", tc.sec, tc.nsec, got, err)
			}
		} else if err != nil || got != tc.want {
			t.Errorf("elapsed (%d,%d) = %v, %v; want %v", tc.sec, tc.nsec, got, err, tc.want)
		}
	}
}
