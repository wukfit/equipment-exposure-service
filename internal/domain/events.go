package domain

import (
	"time"

	"github.com/google/uuid"
)

// ExposureRecorded is the domain event emitted when an exposure is recorded. It
// is the event *fact* (ubiquitous-language) and lives in the domain; the port
// that publishes it (app.EventPublisher) is an application concern.
type ExposureRecorded struct {
	ExposureID  uuid.UUID
	UserID      uuid.UUID
	EquipmentID uuid.UUID
	A8          float64
	Points      float64
	RecordedAt  time.Time
}
