package tockroach_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/zlberto/tockroach"
)

func ExampleNow() {
	expiresAt := time.Date(2026, 1, 1, 12, 5, 0, 0, time.UTC)
	now, err := tockroach.Now()
	if errors.Is(err, tockroach.ErrUnavailable) {
		// Reject the operation or report unavailable. Do not fall back to time.Now.
		return
	}
	if !now.Before(expiresAt) {
		// The deadline is inclusive: equality means expired.
		return
	}
	// Continue only while unexpired.
}

func ExampleNewWithSource() {
	wall := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	elapsed := time.Hour
	clock := tockroach.NewWithSource(func() (time.Time, time.Duration, error) {
		return wall, elapsed, nil
	})
	now, err := clock.Now()
	if err != nil {
		panic(err)
	}
	deadline := now.Add(5 * time.Minute)

	// Simulate ten minutes asleep and a wall clock moved back one hour.
	elapsed += 10 * time.Minute
	wall = wall.Add(-time.Hour)
	now, err = clock.Now()
	if err != nil {
		panic(err)
	}
	fmt.Println(now.Format(time.RFC3339))
	fmt.Println("expired:", !now.Before(deadline))
	// Output:
	// 2026-01-01T12:10:00Z
	// expired: true
}
