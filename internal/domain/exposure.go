package domain

import (
	"time"

	"github.com/google/uuid"
)

type Exposure struct {
	id          uuid.UUID
	userID      uuid.UUID
	equipmentID uuid.UUID
	duration    int
	partial     PartialExposure
	recordedAt  time.Time
}

// NewExposure creates a new exposure, computing its partial exposure from the
// equipment's vibration magnitude and the duration.
func NewExposure(userID, equipmentID uuid.UUID, minutes int, magnitude float64, recordedAt time.Time) (*Exposure, error) {
	if minutes <= 0 {
		return nil, ErrInvalidDuration
	}
	return &Exposure{
		id:          uuid.New(),
		userID:      userID,
		equipmentID: equipmentID,
		duration:    minutes,
		partial:     NewPartialExposure(magnitude, minutes),
		recordedAt:  recordedAt,
	}, nil
}

// NewExposureFromStore reconstitutes an exposure from persisted state without
// recomputing (used by repository adapters).
func NewExposureFromStore(id, userID, equipmentID uuid.UUID, minutes int, partial PartialExposure, recordedAt time.Time) *Exposure {
	return &Exposure{id: id, userID: userID, equipmentID: equipmentID, duration: minutes, partial: partial, recordedAt: recordedAt}
}

func (e *Exposure) ID() uuid.UUID            { return e.id }
func (e *Exposure) UserID() uuid.UUID        { return e.userID }
func (e *Exposure) EquipmentID() uuid.UUID   { return e.equipmentID }
func (e *Exposure) Duration() int            { return e.duration }
func (e *Exposure) Partial() PartialExposure { return e.partial }
func (e *Exposure) RecordedAt() time.Time    { return e.recordedAt }
