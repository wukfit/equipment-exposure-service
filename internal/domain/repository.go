package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ExposureRepository interface {
	Save(ctx context.Context, e *Exposure) error
	GetByID(ctx context.Context, id uuid.UUID) (*Exposure, error)
	List(ctx context.Context) ([]*Exposure, error)
	ListByUserInWindow(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]*Exposure, error)
}

type EquipmentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*EquipmentItem, error)
}

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
}
