package query_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wukfit/equipment-exposure-service/internal/adapters/repository/memory"
	"github.com/wukfit/equipment-exposure-service/internal/app/query"
	"github.com/wukfit/equipment-exposure-service/internal/domain"
	"github.com/wukfit/equipment-exposure-service/internal/seed"
)

func TestGetExposure(t *testing.T) {
	fixedTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	newHandler := func() (*query.GetExposure, *memory.ExposureRepo) {
		exposures := memory.NewExposureRepo()
		return query.NewGetExposure(
			exposures,
			memory.NewUserRepo(seed.Users()...),
			memory.NewEquipmentRepo(seed.Equipment()...),
		), exposures
	}

	t.Run("returns read model with resolved user and equipment", func(t *testing.T) {
		h, exposures := newHandler()

		exp, err := domain.NewExposure(seed.BobbyID, seed.AirCatID, 30, 2.1, fixedTime)
		require.NoError(t, err)
		require.NoError(t, exposures.Save(context.Background(), exp))

		rm, err := h.Handle(context.Background(), exp.ID())
		require.NoError(t, err)

		assert.Equal(t, exp.ID(), rm.Exposure.ID())
		assert.Equal(t, "Bobby Tables", rm.User.Name())
		assert.Equal(t, "AirCat - Drill - 4337", rm.Equipment.Name())
		assert.InDelta(t, 0.525, rm.Exposure.Partial().A8(), 0.001)
		assert.InDelta(t, 4.0, rm.Exposure.Partial().Points(), 0.001)
		assert.Equal(t, 30, rm.Exposure.Duration())
	})

	t.Run("unknown id returns ErrExposureNotFound", func(t *testing.T) {
		h, _ := newHandler()

		_, err := h.Handle(context.Background(), seed.BobbyID) // any uuid not saved
		assert.ErrorIs(t, err, domain.ErrExposureNotFound)
	})
}
