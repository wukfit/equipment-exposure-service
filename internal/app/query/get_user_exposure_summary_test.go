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

func TestGetUserExposureSummary(t *testing.T) {
	base := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	setup := func(t *testing.T) (*query.GetUserExposureSummary, *memory.ExposureRepo) {
		t.Helper()
		users := memory.NewUserRepo(seed.Users()...)
		exposures := memory.NewExposureRepo()
		h := query.NewGetUserExposureSummary(exposures, users)
		return h, exposures
	}

	t.Run("two in-window exposures aggregate correctly", func(t *testing.T) {
		h, exposures := setup(t)

		// Bobby: 30 min @ 2.1 m/s² + 120 min @ 4.0 m/s² (both in window)
		e1, err := domain.NewExposure(seed.BobbyID, seed.AirCatID, 30, 2.1, base)
		require.NoError(t, err)
		require.NoError(t, exposures.Save(ctx, e1))

		e2, err := domain.NewExposure(seed.BobbyID, seed.JCBID, 120, 4.0, base.Add(time.Minute))
		require.NoError(t, err)
		require.NoError(t, exposures.Save(ctx, e2))

		// Alice in-window (should not affect Bobby's summary)
		ea, err := domain.NewExposure(seed.AliceID, seed.AirCatID, 480, 2.1, base)
		require.NoError(t, err)
		require.NoError(t, exposures.Save(ctx, ea))

		// Bobby out-of-window
		eOut, err := domain.NewExposure(seed.BobbyID, seed.AirCatID, 480, 2.1, base.Add(2*time.Hour))
		require.NoError(t, err)
		require.NoError(t, exposures.Save(ctx, eOut))

		result, err := h.Handle(ctx, seed.BobbyID, base.Add(-time.Hour), base.Add(time.Hour))
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "Bobby Tables", result.User.Name())
		assert.InDelta(t, 2.0678, result.Partial.A8(), 0.001)
		assert.Equal(t, float64(68), result.Partial.Points())
	})

	t.Run("unknown user returns ErrUserNotFound", func(t *testing.T) {
		// Handler over an empty user repo, so the lookup misses.
		h := query.NewGetUserExposureSummary(memory.NewExposureRepo(), memory.NewUserRepo())
		_, err := h.Handle(ctx, seed.BobbyID, base.Add(-time.Hour), base.Add(time.Hour))
		assert.ErrorIs(t, err, domain.ErrUserNotFound)
	})

	t.Run("user with no in-window exposures returns zeros", func(t *testing.T) {
		h, _ := setup(t)

		result, err := h.Handle(ctx, seed.BobbyID, base.Add(-time.Hour), base.Add(time.Hour))
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, float64(0), result.Partial.A8())
		assert.Equal(t, float64(0), result.Partial.Points())
	})
}
