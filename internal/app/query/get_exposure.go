package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type GetExposure struct {
	exposures domain.ExposureRepository
	users     domain.UserRepository
	equipment domain.EquipmentRepository
}

func NewGetExposure(e domain.ExposureRepository, u domain.UserRepository, eq domain.EquipmentRepository) *GetExposure {
	return &GetExposure{exposures: e, users: u, equipment: eq}
}

func (h *GetExposure) Handle(ctx context.Context, id uuid.UUID) (*ExposureReadModel, error) {
	exp, err := h.exposures.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// The user/equipment lookups always resolve: the catalog is immutable and the
	// references were validated when the exposure was recorded. If the catalog
	// becomes mutable (Postgres / lifecycle events), a miss here is a referential
	// data-consistency error and should surface as 500, not as a 404 not-found.
	user, err := h.users.GetByID(ctx, exp.UserID())
	if err != nil {
		return nil, err
	}
	equip, err := h.equipment.GetByID(ctx, exp.EquipmentID())
	if err != nil {
		return nil, err
	}
	return &ExposureReadModel{Exposure: exp, User: user, Equipment: equip}, nil
}
