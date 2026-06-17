package memory

import (
	"context"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type EquipmentRepo struct {
	data map[uuid.UUID]*domain.EquipmentItem
}

func NewEquipmentRepo(items ...*domain.EquipmentItem) *EquipmentRepo {
	m := make(map[uuid.UUID]*domain.EquipmentItem, len(items))
	for _, it := range items {
		m[it.ID()] = it
	}
	return &EquipmentRepo{data: m}
}

func (r *EquipmentRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.EquipmentItem, error) {
	it, ok := r.data[id]
	if !ok {
		return nil, domain.ErrEquipmentNotFound
	}
	return it, nil
}
