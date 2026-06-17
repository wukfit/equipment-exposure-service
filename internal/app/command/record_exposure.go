package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/app"
	"github.com/wukfit/equipment-exposure-service/internal/app/query"
	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type RecordExposureInput struct {
	UserID      uuid.UUID
	EquipmentID uuid.UUID
	Duration    int
}

type RecordExposure struct {
	exposures domain.ExposureRepository
	users     domain.UserRepository
	equipment domain.EquipmentRepository
	publisher domain.EventPublisher
	clock     app.Clock
}

func NewRecordExposure(e domain.ExposureRepository, u domain.UserRepository, eq domain.EquipmentRepository, p domain.EventPublisher, c app.Clock) *RecordExposure {
	return &RecordExposure{exposures: e, users: u, equipment: eq, publisher: p, clock: c}
}

func (h *RecordExposure) Handle(ctx context.Context, in RecordExposureInput) (*query.ExposureReadModel, error) {
	user, err := h.users.GetByID(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	equip, err := h.equipment.GetByID(ctx, in.EquipmentID)
	if err != nil {
		return nil, err
	}
	exp, err := domain.NewExposure(user.ID(), equip.ID(), in.Duration, equip.VibrationMagnitude(), h.clock())
	if err != nil {
		return nil, err
	}
	if err := h.exposures.Save(ctx, exp); err != nil {
		return nil, err
	}
	_ = h.publisher.Publish(ctx, domain.ExposureRecorded{
		ExposureID: exp.ID(), UserID: user.ID(), EquipmentID: equip.ID(),
		A8: exp.Partial().A8(), Points: exp.Partial().Points(), RecordedAt: exp.RecordedAt(),
	})
	return &query.ExposureReadModel{Exposure: exp, User: user, Equipment: equip}, nil
}
