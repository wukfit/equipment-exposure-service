package query

import (
	"context"
	"fmt"

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
	// The exposure exists, so its references must resolve. A miss here is a
	// referential data-consistency fault (e.g. a future mutable catalog dropped
	// the record), not a client "not found" — wrap as ErrDataConsistency so the
	// HTTP layer maps it to 500, not 404. (%v, not %w, on the underlying sentinel
	// so it can't match the 404 mappings.)
	user, err := h.users.GetByID(ctx, exp.UserID())
	if err != nil {
		return nil, fmt.Errorf("exposure %s references missing user %s: %w (%v)", exp.ID(), exp.UserID(), domain.ErrDataConsistency, err)
	}
	equip, err := h.equipment.GetByID(ctx, exp.EquipmentID())
	if err != nil {
		return nil, fmt.Errorf("exposure %s references missing equipment %s: %w (%v)", exp.ID(), exp.EquipmentID(), domain.ErrDataConsistency, err)
	}
	return &ExposureReadModel{Exposure: exp, User: user, Equipment: equip}, nil
}
