package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUser(t *testing.T) {
	id := uuid.New()
	u, err := NewUser(id, "Bobby Tables")
	require.NoError(t, err)
	assert.Equal(t, "Bobby Tables", u.Name())

	_, err = NewUser(id, "")
	assert.ErrorIs(t, err, ErrInvalidName)
}
