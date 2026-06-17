package events_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wukfit/equipment-exposure-service/internal/adapters/events"
	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

func TestSlogPublisher(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	pub := events.NewSlogPublisher(logger)

	exposureID := uuid.New()
	err := pub.Publish(context.Background(), domain.ExposureRecorded{
		ExposureID:  exposureID,
		UserID:      uuid.New(),
		EquipmentID: uuid.New(),
		A8:          0.525,
		Points:      4,
		RecordedAt:  time.Now(),
	})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, `"event":"ExposureRecorded"`)
	assert.Contains(t, out, exposureID.String())
}
