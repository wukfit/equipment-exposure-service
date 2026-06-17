package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveWindow(t *testing.T) {
	now := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	clock := Clock(func() time.Time { return now })

	t.Run("both omitted -> trailing 24h", func(t *testing.T) {
		s, e, err := ResolveWindow(clock, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, now.Add(-24*time.Hour), s)
		assert.Equal(t, now, e)
	})
	t.Run("start only -> end=now", func(t *testing.T) {
		start := now.Add(-72 * time.Hour)
		s, e, err := ResolveWindow(clock, &start, nil)
		require.NoError(t, err)
		assert.Equal(t, start, s)
		assert.Equal(t, now, e)
	})
	t.Run("end only -> start=end-24h", func(t *testing.T) {
		end := now.Add(-48 * time.Hour)
		s, e, err := ResolveWindow(clock, nil, &end)
		require.NoError(t, err)
		assert.Equal(t, end.Add(-24*time.Hour), s)
		assert.Equal(t, end, e)
	})
	t.Run("start after end -> error", func(t *testing.T) {
		start, end := now, now.Add(-time.Hour)
		_, _, err := ResolveWindow(clock, &start, &end)
		assert.ErrorIs(t, err, ErrInvalidWindow)
	})
	t.Run("start == end is allowed (empty window)", func(t *testing.T) {
		s, e, err := ResolveWindow(clock, &now, &now)
		require.NoError(t, err)
		assert.Equal(t, now, s)
		assert.Equal(t, now, e)
	})
}
