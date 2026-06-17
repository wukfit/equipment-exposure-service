package app

import "time"

// Clock returns the current time; injected for deterministic tests.
type Clock func() time.Time

// SystemClock returns wall-clock UTC.
func SystemClock() time.Time { return time.Now().UTC() }
