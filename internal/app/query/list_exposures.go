package query

import (
	"context"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type ListExposures struct {
	exposures domain.ExposureRepository
	users     domain.UserRepository
	equipment domain.EquipmentRepository
}

func NewListExposures(e domain.ExposureRepository, u domain.UserRepository, eq domain.EquipmentRepository) *ListExposures {
	return &ListExposures{exposures: e, users: u, equipment: eq}
}

func (h *ListExposures) Handle(ctx context.Context) ([]*ExposureReadModel, error) {
	// List's order (RecordedAt asc, ties by ID) is guaranteed by the repository
	// port contract, so the response order is deterministic across adapters.
	exps, err := h.exposures.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*ExposureReadModel, 0, len(exps))
	for _, exp := range exps {
		rm, err := resolveReadModel(ctx, exp, h.users, h.equipment)
		if err != nil {
			return nil, err
		}
		out = append(out, rm)
	}
	return out, nil
}
