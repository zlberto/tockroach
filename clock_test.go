package tockroach_test

import (
	"errors"
	"math"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/zlberto/tockroach"
)

var epoch = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func TestWallCorrectionsAndSuspend(t *testing.T) {
	wall, elapsed := epoch, time.Hour
	c := tockroach.NewWithSource(func() (time.Time, time.Duration, error) { return wall, elapsed, nil })
	for _, step := range []struct {
		name                string
		wall, elapsed, want time.Duration
	}{
		{"initial", 0, time.Hour, 0},
		{"rollback during suspend", -time.Hour, time.Hour + 5*time.Minute, 5 * time.Minute},
		{"forward correction", 2 * time.Hour, time.Hour + 6*time.Minute, 2 * time.Hour},
		{"reversed correction", 0, time.Hour + 7*time.Minute, 2*time.Hour + time.Minute},
		{"equal samples allowed", 0, time.Hour + 7*time.Minute, 2*time.Hour + time.Minute},
	} {
		t.Run(step.name, func(t *testing.T) {
			wall, elapsed = epoch.Add(step.wall), step.elapsed
			got, err := c.Now()
			if err != nil || !got.Equal(epoch.Add(step.want)) {
				t.Fatalf("Now() = %v, %v; want %v", got, err, epoch.Add(step.want))
			}
		})
	}
}

func TestFailureIsPermanent(t *testing.T) {
	for _, kind := range []string{"source error", "elapsed rollback", "negative elapsed", "zero wall", "year zero", "year 10000", "floor overflow"} {
		t.Run(kind, func(t *testing.T) {
			wall, elapsed := epoch, time.Hour
			var sourceErr error
			calls := 0
			c := tockroach.NewWithSource(func() (time.Time, time.Duration, error) {
				calls++
				return wall, elapsed, sourceErr
			})
			if kind == "floor overflow" {
				wall = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
			}
			if _, err := c.Now(); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "source error":
				sourceErr = errors.New("fake private detail")
			case "elapsed rollback":
				elapsed--
			case "negative elapsed":
				elapsed = -1
			case "zero wall":
				wall = time.Time{}
			case "year zero":
				wall = time.Date(0, 12, 31, 0, 0, 0, 0, time.UTC)
			case "year 10000":
				wall = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
			case "floor overflow":
				elapsed += time.Second
			}
			got, err := c.Now()
			if !got.IsZero() || err != tockroach.ErrUnavailable {
				t.Fatalf("unsafe sample accepted or source error exposed: %v, %v", got, err)
			}
			wall, elapsed, sourceErr = epoch.Add(time.Hour), 2*time.Hour, nil
			got, err = c.Now()
			if !got.IsZero() || !errors.Is(err, tockroach.ErrUnavailable) || calls != 2 {
				t.Fatalf("failed clock recovered or resampled: %v, %v, calls=%d", got, err, calls)
			}
		})
	}
}

func TestFirstSampleFailureIsPermanent(t *testing.T) {
	calls := 0
	c := tockroach.NewWithSource(func() (time.Time, time.Duration, error) {
		calls++
		if calls == 1 {
			return time.Time{}, 0, errors.New("fake failure")
		}
		return epoch, time.Hour, nil
	})
	for range 2 {
		if got, err := c.Now(); !got.IsZero() || err != tockroach.ErrUnavailable {
			t.Fatalf("Now() = %v, %v", got, err)
		}
	}
	if calls != 1 {
		t.Fatalf("resampled failed source %d times", calls)
	}
}

func TestUTCAndNoGoMonotonicReading(t *testing.T) {
	for _, wall := range []time.Time{time.Now(), epoch.In(time.FixedZone("offset", 3600))} {
		c := tockroach.NewWithSource(func() (time.Time, time.Duration, error) { return wall, 0, nil })
		got, err := c.Now()
		if err != nil || got != wall.UTC() {
			t.Fatalf("Now() = %v, %v; want UTC without monotonic reading %v", got, err, wall.UTC())
		}
	}
}

func TestFullElapsedRange(t *testing.T) {
	elapsed := time.Duration(0)
	c := tockroach.NewWithSource(func() (time.Time, time.Duration, error) { return epoch, elapsed, nil })
	if _, err := c.Now(); err != nil {
		t.Fatal(err)
	}
	elapsed = time.Duration(math.MaxInt64)
	got, err := c.Now()
	if err != nil || !got.Equal(time.Date(2318, 4, 13, 11, 47, 16, 854775807, time.UTC)) {
		t.Fatalf("maximum duration overflowed: %v, %v", got, err)
	}
}

func TestIndependentClocks(t *testing.T) {
	wall := epoch
	source := func() (time.Time, time.Duration, error) { return wall, 0, nil }
	first := tockroach.NewWithSource(source)
	if _, err := first.Now(); err != nil {
		t.Fatal(err)
	}
	wall = epoch.Add(-time.Hour)
	second := tockroach.NewWithSource(source)
	gotFirst, errFirst := first.Now()
	gotSecond, errSecond := second.Now()
	if errFirst != nil || errSecond != nil || !gotFirst.Equal(epoch) || !gotSecond.Equal(wall) {
		t.Fatalf("clock state leaked: first=%v (%v), second=%v (%v)", gotFirst, errFirst, gotSecond, errSecond)
	}
}

func TestConcurrentCallsSerializeSourceAndFloor(t *testing.T) {
	// The source deliberately has no lock: Clock must serialize sampling too.
	elapsed := time.Duration(0)
	c := tockroach.NewWithSource(func() (time.Time, time.Duration, error) {
		elapsed++
		return epoch, elapsed, nil
	})
	const n = 256
	results := make(chan time.Time, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := c.Now()
			if err != nil {
				t.Error(err)
				return
			}
			results <- got
		}()
	}
	wg.Wait()
	close(results)
	seen := make(map[time.Time]bool)
	for got := range results {
		if seen[got] || got.Before(epoch) || got.After(epoch.Add(n-1)) {
			t.Fatalf("lost elapsed time: %v", got)
		}
		seen[got] = true
	}
	if len(seen) != n {
		t.Fatalf("got %d distinct samples, want %d", len(seen), n)
	}
}

func TestSystemClock(t *testing.T) {
	var c tockroach.Clock
	for _, now := range []func() (time.Time, error){tockroach.Now, c.Now, tockroach.NewWithSource(nil).Now} {
		first, err := now()
		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
			if !first.IsZero() || !errors.Is(err, tockroach.ErrUnavailable) {
				t.Fatalf("unsupported platform accepted: %v, %v", first, err)
			}
			continue
		}
		if err != nil || first.IsZero() || first.Location() != time.UTC {
			t.Fatalf("system sample = %v, %v", first, err)
		}
		second, err := now()
		if err != nil || second.Before(first) {
			t.Fatalf("system clock regressed: %v -> %v, %v", first, second, err)
		}
	}
}
