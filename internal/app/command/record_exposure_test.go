package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wukfit/equipment-exposure-service/internal/adapters/repository/memory"
	"github.com/wukfit/equipment-exposure-service/internal/app"
	"github.com/wukfit/equipment-exposure-service/internal/app/command"
	"github.com/wukfit/equipment-exposure-service/internal/domain"
	"github.com/wukfit/equipment-exposure-service/internal/seed"
)

type spyPublisher struct{ events []domain.ExposureRecorded }

func (s *spyPublisher) Publish(_ context.Context, e domain.ExposureRecorded) error {
	s.events = append(s.events, e)
	return nil
}

func TestRecordExposure(t *testing.T) {
	fixed := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := app.Clock(func() time.Time { return fixed })

	t.Run("happy path", func(t *testing.T) {
		pub := &spyPublisher{}
		h := command.NewRecordExposure(
			memory.NewExposureRepo(),
			memory.NewUserRepo(seed.Users()...),
			memory.NewEquipmentRepo(seed.Equipment()...),
			pub, clock,
		)

		rm, err := h.Handle(context.Background(), command.RecordExposureInput{
			UserID: seed.BobbyID, EquipmentID: seed.AirCatID, Duration: 30,
		})
		require.NoError(t, err)
		assert.InDelta(t, 0.525, rm.Exposure.Partial().A8(), 0.001)
		assert.Equal(t, fixed, rm.Exposure.RecordedAt())
		require.Len(t, pub.events, 1)
		assert.Equal(t, rm.Exposure.ID(), pub.events[0].ExposureID)
	})

	t.Run("unknown user returns ErrUserNotFound", func(t *testing.T) {
		pub := &spyPublisher{}
		h := command.NewRecordExposure(
			memory.NewExposureRepo(),
			memory.NewUserRepo(seed.Users()...),
			memory.NewEquipmentRepo(seed.Equipment()...),
			pub, clock,
		)

		_, err := h.Handle(context.Background(), command.RecordExposureInput{
			UserID: uuid.New(), EquipmentID: seed.AirCatID, Duration: 30,
		})
		assert.ErrorIs(t, err, domain.ErrUserNotFound)
	})

	t.Run("unknown equipment returns ErrEquipmentNotFound", func(t *testing.T) {
		pub := &spyPublisher{}
		h := command.NewRecordExposure(
			memory.NewExposureRepo(),
			memory.NewUserRepo(seed.Users()...),
			memory.NewEquipmentRepo(seed.Equipment()...),
			pub, clock,
		)

		_, err := h.Handle(context.Background(), command.RecordExposureInput{
			UserID: seed.BobbyID, EquipmentID: uuid.New(), Duration: 30,
		})
		assert.ErrorIs(t, err, domain.ErrEquipmentNotFound)
	})

	t.Run("duration 0 returns ErrInvalidDuration", func(t *testing.T) {
		pub := &spyPublisher{}
		h := command.NewRecordExposure(
			memory.NewExposureRepo(),
			memory.NewUserRepo(seed.Users()...),
			memory.NewEquipmentRepo(seed.Equipment()...),
			pub, clock,
		)

		_, err := h.Handle(context.Background(), command.RecordExposureInput{
			UserID: seed.BobbyID, EquipmentID: seed.AirCatID, Duration: 0,
		})
		assert.ErrorIs(t, err, domain.ErrInvalidDuration)
	})
}
