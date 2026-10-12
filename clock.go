package tockroach

import (
	"errors"
	"sync"
	"time"
)

// ErrUnavailable means the clock can no longer supply a valid time.
// Once returned, every subsequent call on that clock returns this error.
// Underlying source errors are deliberately not exposed.
var ErrUnavailable = errors.New("tockroach: clock unavailable")

// Clock combines wall time with elapsed time that includes system suspension.
// Its zero value uses the system clocks. It is safe for concurrent use and must
// not be copied after first use. Each Clock has its own floor and failure latch.
type Clock struct {
	mu      sync.Mutex
	source  func() (time.Time, time.Duration, error)
	wall    time.Time
	elapsed time.Duration
	failed  bool
}

// NewWithSource creates an independent clock using source; nil uses the system
// clocks. Source must return wall time and a nonnegative, nondecreasing elapsed
// reading from a fixed origin, including suspension. Read elapsed before wall
// time so later samples retain any pause between the reads. The source runs
// under the clock's lock and must not call back into the same clock. Panics are
// not recovered. Tests can inject a source without changing the shared clock.
func NewWithSource(source func() (time.Time, time.Duration, error)) *Clock {
	return &Clock{source: source}
}

// Now returns UTC time without a Go monotonic reading. Each result is at least
// the previous result plus the elapsed time since the previous sample. Forward
// wall corrections advance this floor; backward corrections cannot lower it.
// Guarantees apply to sampled readings: a pause after the elapsed read can
// make the result stale before it reaches the caller.
// An invalid sample or elapsed rollback permanently fails this clock. Valid
// times have UTC years 1 through 9999 and are not the zero time.
func (c *Clock) Now() (time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failed {
		return time.Time{}, ErrUnavailable
	}
	source := c.source
	if source == nil {
		source = systemTime
	}
	wall, elapsed, err := source()
	wall = wall.UTC()
	if err != nil || !validTime(wall) || elapsed < 0 || !c.wall.IsZero() && elapsed < c.elapsed {
		c.failed = true
		return time.Time{}, ErrUnavailable
	}
	if !c.wall.IsZero() {
		floor := c.wall.Add(elapsed - c.elapsed)
		if wall.Before(floor) {
			wall = floor
		}
	}
	if !validTime(wall) {
		c.failed = true
		return time.Time{}, ErrUnavailable
	}
	c.wall, c.elapsed = wall, elapsed
	return wall, nil
}

var shared Clock

// Now uses one shared process-local floor and failure latch. Its first call
// establishes the floor. A failure lasts until process restart.
func Now() (time.Time, error) { return shared.Now() }

func validTime(t time.Time) bool { return !t.IsZero() && t.Year() >= 1 && t.Year() <= 9999 }
