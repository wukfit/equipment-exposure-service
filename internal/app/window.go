package app

import (
	"errors"
	"time"
)

const defaultWindow = 24 * time.Hour

// ErrInvalidWindow is returned when starting_at is strictly after ending_at.
var ErrInvalidWindow = errors.New("starting_at must not be after ending_at")

// ResolveWindow applies the default-window rules (design §3): both omitted ->
// trailing 24h [now-24h, now); start only -> end=now; end only -> start=end-24h.
// start == end is allowed (empty window -> zeros). start strictly after end is
// ErrInvalidWindow.
func ResolveWindow(clock Clock, start, end *time.Time) (time.Time, time.Time, error) {
	now := clock()
	var s, e time.Time
	switch {
	case start == nil && end == nil:
		s, e = now.Add(-defaultWindow), now
	case start != nil && end == nil:
		s, e = *start, now
	case start == nil && end != nil:
		s, e = end.Add(-defaultWindow), *end
	default:
		s, e = *start, *end
	}
	if s.After(e) {
		return time.Time{}, time.Time{}, ErrInvalidWindow
	}
	return s, e, nil
}
