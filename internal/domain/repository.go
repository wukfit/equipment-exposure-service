package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ExposureRepository interface {
	Save(ctx context.Context, e *Exposure) error
	GetByID(ctx context.Context, id uuid.UUID) (*Exposure, error)
	// List returns all exposures in a deterministic order: RecordedAt ascending,
	// ties broken by ID ascending. This ordering is part of the port contract
	// (enforced by the shared repository contract test) so every adapter — and
	// the list endpoint built on it — is deterministic, not just the in-memory one.
	List(ctx context.Context) ([]*Exposure, error)
	// ListByUserInWindow returns the user's exposures whose RecordedAt falls in
	// the half-open window [start, end), in the same RecordedAt-then-ID order as List.
	ListByUserInWindow(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]*Exposure, error)
}

type EquipmentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*EquipmentItem, error)
}

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
}
