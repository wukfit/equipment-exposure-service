package contracttest

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

// RunExposureRepository exercises any ExposureRepository implementation.
func RunExposureRepository(t *testing.T, newRepo func() domain.ExposureRepository) {
	ctx := context.Background()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("save then get", func(t *testing.T) {
		repo := newRepo()
		e, _ := domain.NewExposure(uuid.New(), uuid.New(), 30, 2.1, base)
		require.NoError(t, repo.Save(ctx, e))
		got, err := repo.GetByID(ctx, e.ID())
		require.NoError(t, err)
		assert.Equal(t, e.ID(), got.ID())
	})

	t.Run("get missing returns ErrExposureNotFound", func(t *testing.T) {
		repo := newRepo()
		_, err := repo.GetByID(ctx, uuid.New())
		assert.ErrorIs(t, err, domain.ErrExposureNotFound)
	})

	t.Run("window filters by user and time", func(t *testing.T) {
		repo := newRepo()
		user := uuid.New()
		other := uuid.New()
		inWindow, _ := domain.NewExposure(user, uuid.New(), 30, 2.1, base)
		before, _ := domain.NewExposure(user, uuid.New(), 30, 2.1, base.Add(-48*time.Hour))
		otherUser, _ := domain.NewExposure(other, uuid.New(), 30, 2.1, base)
		for _, e := range []*domain.Exposure{inWindow, before, otherUser} {
			require.NoError(t, repo.Save(ctx, e))
		}
		got, err := repo.ListByUserInWindow(ctx, user, base.Add(-24*time.Hour), base.Add(time.Hour))
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, inWindow.ID(), got[0].ID())
	})

	t.Run("concurrent saves are safe", func(t *testing.T) {
		repo := newRepo()
		done := make(chan struct{})
		for i := 0; i < 50; i++ {
			go func() {
				e, _ := domain.NewExposure(uuid.New(), uuid.New(), 30, 2.1, base)
				_ = repo.Save(ctx, e)
				done <- struct{}{}
			}()
		}
		for i := 0; i < 50; i++ {
			<-done
		}
		all, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Len(t, all, 50)
	})
}
