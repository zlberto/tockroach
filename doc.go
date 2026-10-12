// Package tockroach provides a clock for expiry checks that keeps advancing
// through system suspension and observed wall-clock rollback.
//
// Use Now to share a process-local floor, or keep a long-lived Clock for an
// independent floor. Always handle ErrUnavailable; falling back to time.Now
// defeats the guarantee. Compare deadlines with values from the same clock.
//
// Tockroach does not establish accurate UTC, persist state across restarts, or
// detect an unobserved wall-clock adjustment. Forward corrections and sampling
// delays can cause early expiry. It is not a clock for scheduling, not-before
// checks, billing, distributed ordering, or measuring precise durations.
//
// System sources support Linux (CLOCK_BOOTTIME) and macOS
// (CLOCK_MONOTONIC_RAW). Other platforms return ErrUnavailable; a custom source
// can be supplied with NewWithSource. Returned values are UTC and contain no Go
// monotonic reading. The package creates no goroutines or timers.
package tockroach
