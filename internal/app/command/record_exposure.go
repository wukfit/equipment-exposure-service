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
	exposures app.ExposureStore
	users     app.UserDirectory
	equipment app.EquipmentCatalog
	publisher app.EventPublisher
	clock     app.Clock
}

func NewRecordExposure(e app.ExposureStore, u app.UserDirectory, eq app.EquipmentCatalog, p app.EventPublisher, c app.Clock) *RecordExposure {
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
	// Event publication is intentionally best-effort and non-blocking: the
	// exposure is already durably saved, so a failed emit must not fail the
	// request. The discard is a deliberate decision, not an oversight — a real
	// broker adapter would add retry/buffering behind this same port.
	_ = h.publisher.Publish(ctx, domain.ExposureRecorded{
		ExposureID: exp.ID(), UserID: user.ID(), EquipmentID: equip.ID(),
		A8: exp.Partial().A8(), Points: exp.Partial().Points(), RecordedAt: exp.RecordedAt(),
	})
	return &query.ExposureReadModel{Exposure: exp, User: user, Equipment: equip}, nil
}
