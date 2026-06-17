package query_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wukfit/equipment-exposure-service/internal/adapters/repository/memory"
	"github.com/wukfit/equipment-exposure-service/internal/app/query"
	"github.com/wukfit/equipment-exposure-service/internal/domain"
	"github.com/wukfit/equipment-exposure-service/internal/seed"
)

func TestListExposures(t *testing.T) {
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	newHandler := func() (*query.ListExposures, *memory.ExposureRepo) {
		exposures := memory.NewExposureRepo()
		return query.NewListExposures(
			exposures,
			memory.NewUserRepo(seed.Users()...),
			memory.NewEquipmentRepo(seed.Equipment()...),
		), exposures
	}

	t.Run("empty repo returns empty slice, no error", func(t *testing.T) {
		h, _ := newHandler()

		rms, err := h.Handle(context.Background())
		require.NoError(t, err)
		assert.Empty(t, rms)
	})

	t.Run("two exposures returned ordered by recordedAt ascending with resolved names", func(t *testing.T) {
		h, exposures := newHandler()

		expEarlier, err := domain.NewExposure(seed.BobbyID, seed.AirCatID, 30, 2.1, base)
		require.NoError(t, err)
		expLater, err := domain.NewExposure(seed.AliceID, seed.JCBID, 60, 4.0, base.Add(time.Hour))
		require.NoError(t, err)

		require.NoError(t, exposures.Save(context.Background(), expEarlier))
		require.NoError(t, exposures.Save(context.Background(), expLater))

		rms, err := h.Handle(context.Background())
		require.NoError(t, err)
		require.Len(t, rms, 2)

		assert.Equal(t, expEarlier.ID(), rms[0].Exposure.ID())
		assert.Equal(t, "Bobby Tables", rms[0].User.Name())
		assert.Equal(t, "AirCat - Drill - 4337", rms[0].Equipment.Name())

		assert.Equal(t, expLater.ID(), rms[1].Exposure.ID())
		assert.Equal(t, "Alice Stone", rms[1].User.Name())
		assert.Equal(t, "JCB - Hydraulic Breaker - CEJCBHM25", rms[1].Equipment.Name())
	})

	t.Run("dangling user reference is data-consistency error, not user-not-found", func(t *testing.T) {
		h, exposures := newHandler()

		exp, err := domain.NewExposure(uuid.New(), seed.AirCatID, 30, 2.1, base)
		require.NoError(t, err)
		require.NoError(t, exposures.Save(context.Background(), exp))

		_, err = h.Handle(context.Background())
		assert.True(t, errors.Is(err, domain.ErrDataConsistency))
		assert.False(t, errors.Is(err, domain.ErrUserNotFound))
	})

	t.Run("dangling equipment reference is data-consistency error, not equipment-not-found", func(t *testing.T) {
		h, exposures := newHandler()

		exp, err := domain.NewExposure(seed.BobbyID, uuid.New(), 30, 2.1, base)
		require.NoError(t, err)
		require.NoError(t, exposures.Save(context.Background(), exp))

		_, err = h.Handle(context.Background())
		assert.True(t, errors.Is(err, domain.ErrDataConsistency))
		assert.False(t, errors.Is(err, domain.ErrEquipmentNotFound))
	})
}
