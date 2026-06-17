package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type ExposureRepo struct {
	mu   sync.RWMutex
	data map[uuid.UUID]*domain.Exposure
}

func NewExposureRepo() *ExposureRepo {
	return &ExposureRepo{data: make(map[uuid.UUID]*domain.Exposure)}
}

func (r *ExposureRepo) Save(_ context.Context, e *domain.Exposure) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[e.ID()] = e
	return nil
}

func (r *ExposureRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Exposure, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.data[id]
	if !ok {
		return nil, domain.ErrExposureNotFound
	}
	return e, nil
}

func (r *ExposureRepo) List(_ context.Context) ([]*domain.Exposure, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*domain.Exposure, 0, len(r.data))
	for _, e := range r.data {
		out = append(out, e)
	}
	sortByRecordedAt(out)
	return out, nil
}

func (r *ExposureRepo) ListByUserInWindow(_ context.Context, userID uuid.UUID, start, end time.Time) ([]*domain.Exposure, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*domain.Exposure
	for _, e := range r.data {
		if e.UserID() != userID {
			continue
		}
		// half-open window [start, end): an instant on a boundary isn't double-counted
		if e.RecordedAt().Before(start) || !e.RecordedAt().Before(end) {
			continue
		}
		out = append(out, e)
	}
	sortByRecordedAt(out)
	return out, nil
}

// sortByRecordedAt gives deterministic ordering: by RecordedAt, then ID as a
// stable tiebreaker.
func sortByRecordedAt(xs []*domain.Exposure) {
	sort.Slice(xs, func(i, j int) bool {
		if xs[i].RecordedAt().Equal(xs[j].RecordedAt()) {
			return xs[i].ID().String() < xs[j].ID().String()
		}
		return xs[i].RecordedAt().Before(xs[j].RecordedAt())
	})
}
