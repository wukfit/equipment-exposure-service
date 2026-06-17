package query

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/app"
	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type UserExposureSummary struct {
	User    *domain.User
	Partial domain.PartialExposure
}

type GetUserExposureSummary struct {
	exposures app.ExposureStore
	users     app.UserDirectory
}

func NewGetUserExposureSummary(e app.ExposureStore, u app.UserDirectory) *GetUserExposureSummary {
	return &GetUserExposureSummary{exposures: e, users: u}
}

func (h *GetUserExposureSummary) Handle(ctx context.Context, userID uuid.UUID, start, end time.Time) (*UserExposureSummary, error) {
	// userID is the REQUESTED resource here, so a miss is a genuine 404 (unlike
	// the read-side reference resolution in get_exposure — do NOT wrap as
	// ErrDataConsistency).
	user, err := h.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	exps, err := h.exposures.ListByUserInWindow(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}
	parts := make([]domain.PartialExposure, 0, len(exps))
	for _, e := range exps {
		parts = append(parts, e.Partial())
	}
	return &UserExposureSummary{User: user, Partial: domain.Aggregate(parts)}, nil
}
