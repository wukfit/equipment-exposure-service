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

	t.Run("half-open window boundary: start included, end excluded", func(t *testing.T) {
		// Pins the documented [start, end) rule (design §3) so an adapter using
		// inclusive-end (or exclusive-start) semantics cannot pass this contract.
		repo := newRepo()
		user := uuid.New()
		start := base
		end := base.Add(2 * time.Hour)

		atStart, _ := domain.NewExposure(user, uuid.New(), 30, 2.1, start)                          // included: recordedAt == start
		justBeforeEnd, _ := domain.NewExposure(user, uuid.New(), 30, 2.1, end.Add(-time.Nanosecond)) // included: recordedAt < end
		atEnd, _ := domain.NewExposure(user, uuid.New(), 30, 2.1, end)                               // excluded: recordedAt == end
		justBeforeStart, _ := domain.NewExposure(user, uuid.New(), 30, 2.1, start.Add(-time.Nanosecond)) // excluded: recordedAt < start
		for _, e := range []*domain.Exposure{atStart, justBeforeEnd, atEnd, justBeforeStart} {
			require.NoError(t, repo.Save(ctx, e))
		}

		got, err := repo.ListByUserInWindow(ctx, user, start, end)
		require.NoError(t, err)

		ids := make(map[uuid.UUID]bool, len(got))
		for _, e := range got {
			ids[e.ID()] = true
		}
		assert.True(t, ids[atStart.ID()], "exposure exactly at start must be included ([start, end))")
		assert.True(t, ids[justBeforeEnd.ID()], "exposure just before end must be included")
		assert.False(t, ids[atEnd.ID()], "exposure exactly at end must be excluded ([start, end))")
		assert.False(t, ids[justBeforeStart.ID()], "exposure before start must be excluded")
		assert.Len(t, got, 2)
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
