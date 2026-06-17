package domain

import "github.com/google/uuid"

type EquipmentItem struct {
	id                 uuid.UUID
	name               string
	vibrationMagnitude float64
}

func NewEquipmentItem(id uuid.UUID, name string, magnitude float64) (*EquipmentItem, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	if magnitude <= 0 {
		return nil, ErrInvalidMagnitude
	}
	return &EquipmentItem{id: id, name: name, vibrationMagnitude: magnitude}, nil
}

func (e *EquipmentItem) ID() uuid.UUID               { return e.id }
func (e *EquipmentItem) Name() string                { return e.name }
func (e *EquipmentItem) VibrationMagnitude() float64 { return e.vibrationMagnitude }
