package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEquipmentItem(t *testing.T) {
	id := uuid.New()
	e, err := NewEquipmentItem(id, "AirCat - Drill - 4337", 2.1)
	require.NoError(t, err)
	assert.Equal(t, id, e.ID())
	assert.Equal(t, 2.1, e.VibrationMagnitude())

	_, err = NewEquipmentItem(id, "", 2.1)
	assert.ErrorIs(t, err, ErrInvalidName)
	_, err = NewEquipmentItem(id, "x", 0)
	assert.ErrorIs(t, err, ErrInvalidMagnitude)
}
