package memory

import (
	"context"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type UserRepo struct {
	data map[uuid.UUID]*domain.User
}

func NewUserRepo(users ...*domain.User) *UserRepo {
	m := make(map[uuid.UUID]*domain.User, len(users))
	for _, u := range users {
		m[u.ID()] = u
	}
	return &UserRepo{data: m}
}

func (r *UserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.data[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}
