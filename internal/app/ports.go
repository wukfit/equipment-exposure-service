package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

// The application layer owns its ports: each interface is defined here, beside
// the use cases that consume it, rather than in the domain. The domain stays
// pure (entities, the PartialExposure VO, the HAVS calc, sentinel errors, and
// the ExposureRecorded event fact) and depends on nothing. Persistence and event
// publication are application concerns, so the contracts for them live here and
// the adapters implement them — keeping the dependency rule adapters → app →
// domain, while staying idiomatic Go (consumer-defined interfaces).

// ExposureStore persists and retrieves exposures. This service owns exposure
// data, so the store is read+write. List returns all exposures in a
// deterministic order — RecordedAt ascending, ties broken by ID ascending — and
// that order is part of the contract (enforced by the shared repository contract
// suite), so every adapter, and the list endpoint built on it, is deterministic.
type ExposureStore interface {
	Save(ctx context.Context, e *domain.Exposure) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Exposure, error)
	List(ctx context.Context) ([]*domain.Exposure, error)
	// ListByUserInWindow returns the user's exposures whose RecordedAt falls in
	// the half-open window [start, end), in the same RecordedAt-then-ID order as List.
	ListByUserInWindow(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]*domain.Exposure, error)
}

// UserDirectory resolves users by ID. Users are reference data owned by another
// bounded context (they would arrive via UserCreated/UserDeactivated events), so
// this is a read-only lookup, not a repository this service writes to.
type UserDirectory interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// EquipmentCatalog resolves equipment by ID. Like users, equipment is reference
// data owned elsewhere (EquipmentRegistered/EquipmentUpdated events), so it is
// read-only.
type EquipmentCatalog interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.EquipmentItem, error)
}

// EventPublisher emits domain events to downstream consumers. The use case owns
// this port because publishing is an application concern; the event fact itself
// (domain.ExposureRecorded) belongs to the domain.
type EventPublisher interface {
	Publish(ctx context.Context, event domain.ExposureRecorded) error
}
