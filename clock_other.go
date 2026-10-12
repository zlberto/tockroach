//go:build (!linux && !darwin) || android || ios

package tockroach

import "time"

func elapsedTime() (time.Duration, error) { return 0, ErrUnavailable }
