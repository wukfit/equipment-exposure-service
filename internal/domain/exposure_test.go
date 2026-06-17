package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExposure(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	e, err := NewExposure(uuid.New(), uuid.New(), 30, 2.1, now)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, e.ID())
	assert.Equal(t, 30, e.Duration())
	assert.InDelta(t, 0.525, e.Partial().A8(), 0.001)
	assert.Equal(t, now, e.RecordedAt())

	_, err = NewExposure(uuid.New(), uuid.New(), 0, 2.1, now)
	assert.ErrorIs(t, err, ErrInvalidDuration)
}
