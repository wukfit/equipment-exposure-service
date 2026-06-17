package events

import (
	"context"
	"log/slog"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type SlogPublisher struct{ logger *slog.Logger }

func NewSlogPublisher(l *slog.Logger) *SlogPublisher { return &SlogPublisher{logger: l} }

func (p *SlogPublisher) Publish(ctx context.Context, e domain.ExposureRecorded) error {
	p.logger.InfoContext(ctx, "exposure.recorded",
		slog.String("event", "ExposureRecorded"),
		slog.String("exposure_id", e.ExposureID.String()),
		slog.String("user_id", e.UserID.String()),
		slog.String("equipment_id", e.EquipmentID.String()),
		slog.Float64("a8", e.A8),
		slog.Float64("points", e.Points),
		slog.Time("recorded_at", e.RecordedAt),
	)
	return nil
}
