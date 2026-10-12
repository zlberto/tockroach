# tockroach

**A tiny Go expiry clock that survives naps and time travel.**

Tockroach combines UTC wall time with elapsed time that includes system sleep.
Once it observes time moving forward, moving the wall clock backward cannot
make that time available again. Useful for checking lease expiry, cache
freshness, and short-lived permissions when late expiry is worse than early
expiry.

One package. One dependency (`golang.org/x/sys`). No goroutines, timers,
configuration, or network calls. Go 1.25 or later.

## Status and installation

Initial extraction from Kadomori's internal clock. The module path is
`github.com/zlberto/tockroach`; this checkout is not yet a published release.
Once the repository is public:

```sh
go get github.com/zlberto/tockroach
```

## Use it

```go
now, err := tockroach.Now()
if err != nil {
    return err // Fail closed; do not substitute time.Now().
}
if !now.Before(expiresAt) {
    return errExpired
}
```

For a relative deadline, establish it with `now.Add(ttl)` and check it with
the same clock. Sample again at the point where expiry matters; a prior check
does not make later work safe automatically.

`Now()` shares one process-local floor and failure latch. A zero-value `Clock`
uses the same system sources with its own independent state:

```go
var clock tockroach.Clock // Keep this instance for the lifetime of its deadlines.
now, err := clock.Now()
```

The first call establishes the floor. Do not construct a clock per request.
Clocks are safe for concurrent use and must not be copied after first use.
There is deliberately no reset method. Replacing a clock also replaces its
history; doing so to recover from a failure can revive expired state.

## What it guarantees

For each successful sample after the first:

```text
returned time = max(wall time, previous returned time + elapsed since last sample)
```

| Event | Result |
| --- | --- |
| System sleeps for ten minutes | Elapsed time includes those ten minutes |
| Wall clock moves backward | The floor keeps advancing |
| Wall clock moves forward | The observed correction advances the floor |
| That forward correction is reversed | The floor retains the correction |
| Source errors, elapsed time regresses, or a sample is invalid | This clock permanently returns `ErrUnavailable` |
| Process restarts or a new clock is created | A new history begins on its first call |

Equal consecutive readings are allowed. Returned values use UTC and have no
Go monotonic component, so Go comparisons use the expiry time you see.
Valid wall times have UTC years 1–9999 and exclude the zero time. Elapsed
readings must be nonnegative and fit in a `time.Duration` (about 292 years).
Overflow fails closed. Underlying source errors are not exposed or logged.

## Why not just `time.Now()`?

Go's monotonic component protects duration comparisons from wall-clock jumps,
but it can stop during sleep and is absent from parsed or serialized deadlines.
See the [Go time documentation](https://go.dev/pkg/time/#hdr-Monotonic_Clocks).

Tockroach uses a suspend-inclusive source and retains forward corrections
across calls. The system source reads elapsed time **before** wall time. The
reads are not atomic: pairing wall time with an earlier elapsed reading lets
later samples retain a pause between those reads, and can cause early expiry.
Reading them in reverse order can instead lose that pause after a later
wall-clock rollback. Guarantees apply to the sampled readings: a pause after
the elapsed read can still make that call's result stale before it reaches the
caller. This is not a guarantee of freshness at return or at a later use.

| Platform | System source |
| --- | --- |
| Linux | [`CLOCK_BOOTTIME`](https://www.kernel.org/doc/html/v5.10/core-api/timekeeping.html) |
| macOS | [`CLOCK_MONOTONIC_RAW`](https://developer.apple.com/documentation/kernel/1646199-mach_continuous_time) |
| Other operating systems | `ErrUnavailable`, unless a custom source is supplied |

CI runs tests on Linux and macOS and verifies rejection of system sources on
Windows. The adapters need no cgo. Real machine suspend/resume behavior is an
OS contract; tests simulate suspension without putting CI machines to sleep.

## Test your application

Applications usually need only an injected `func() (time.Time, error)`; pass
`tockroach.Now` in production. No clock interface or mocking package is needed.

To exercise the floor itself, inject both source readings:

```go
wall := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
elapsed := time.Hour
clock := tockroach.NewWithSource(func() (time.Time, time.Duration, error) {
    return wall, elapsed, nil
})

first, err := clock.Now()
if err != nil {
    return err
}
deadline := first.Add(5 * time.Minute)

wall = wall.Add(-time.Hour)
elapsed += 10 * time.Minute // Simulated sleep.
now, err := clock.Now()
if err != nil {
    return err
}
fmt.Println(!now.Before(deadline)) // true
```

`NewWithSource(nil)` uses the system source. Custom sources must supply elapsed
time from one fixed origin, count suspension, and sample elapsed before wall
time. Sampling is serialized within one Clock; synchronize externally if
sharing mutable source state across clocks or goroutines. A source must not
call back into the same Clock. Source panics are not recovered.

## Limits worth understanding

- **It does not discover correct UTC.** An initially wrong wall clock stays
  wrong until a forward correction is observed. It is not protection against
  a compromised OS or a malicious custom source.
- **It remembers only observed corrections, inside one clock's lifetime.**
  Restarts, VM snapshots, process checkpoint/restore, and unsampled jumps have
  no durable protection. Applications must reestablish authoritative state
  after restart; this library does not persist or synchronize a floor.
- **It deliberately permits early expiry.** A forward wall jump, sampling
  delay, or elapsed-clock drift can put the floor ahead of actual UTC; that
  offset need not diminish. The library provides no error bound or NTP logic.
- **An expiry clock is not a general trust oracle.** Do not use it to prove
  a not-before condition, for billing, exact duration measurements, distributed
  ordering, or as a blanket replacement for time.Now. An ahead-of-UTC clock
  can incorrectly satisfy a not-before check.
- **It supplies readings, not wakeups.** Go timers, time.Sleep and context
  deadlines retain their normal behavior. Recheck expiry after waiting. Use
  `deadline.Sub(now)` with this clock's `now`; `time.Until(deadline)` samples
  the ordinary wall clock and loses this floor.

## Development

```sh
make check       # Formatting, vet, build, race tests
make bench       # Serial and concurrent reads, with allocations
```

The tests cover rollback, suspension, reversed forward corrections, permanent
failure, sampling delays, overflow, UTC normalization, independent clocks, and
concurrency. CI also tests the minimum Go version. Cross-compilation does not
substitute for running a platform's tests.

Keep the API focused on expiry. Windows support is a reasonable future addition
if backed by a verified suspend-inclusive source and native tests. Timers, fake
clock frameworks, persistence, NTP, and metrics integrations are outside the
initial scope.

## License

MIT, Copyright (c) 2026 Alberto Urena. See [LICENSE](LICENSE).
