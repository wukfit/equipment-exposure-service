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
