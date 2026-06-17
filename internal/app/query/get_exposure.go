package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/app"
)

type GetExposure struct {
	exposures app.ExposureStore
	users     app.UserDirectory
	equipment app.EquipmentCatalog
}

func NewGetExposure(e app.ExposureStore, u app.UserDirectory, eq app.EquipmentCatalog) *GetExposure {
	return &GetExposure{exposures: e, users: u, equipment: eq}
}

func (h *GetExposure) Handle(ctx context.Context, id uuid.UUID) (*ExposureReadModel, error) {
	exp, err := h.exposures.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return resolveReadModel(ctx, exp, h.users, h.equipment)
}
