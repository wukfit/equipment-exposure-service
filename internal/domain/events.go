package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ExposureRecorded struct {
	ExposureID  uuid.UUID
	UserID      uuid.UUID
	EquipmentID uuid.UUID
	A8          float64
	Points      float64
	RecordedAt  time.Time
}

type EventPublisher interface {
	Publish(ctx context.Context, event ExposureRecorded) error
}
