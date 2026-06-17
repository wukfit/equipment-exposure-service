package query

import (
	"context"
	"fmt"

	"github.com/wukfit/equipment-exposure-service/internal/app"
	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

// ExposureReadModel is the composed read-side projection of an exposure with
// its associated user and equipment, used to build the embedded API response.
type ExposureReadModel struct {
	Exposure  *domain.Exposure
	User      *domain.User
	Equipment *domain.EquipmentItem
}

// resolveReadModel composes an exposure with its user and equipment. The
// exposure already exists, so a missing reference is a referential
// data-consistency fault (not a client not-found): wrap as ErrDataConsistency
// (via %v on the underlying sentinel so it can't match the 404 mappings) so the
// HTTP layer maps it to 500.
func resolveReadModel(ctx context.Context, exp *domain.Exposure, users app.UserDirectory, equipment app.EquipmentCatalog) (*ExposureReadModel, error) {
	user, err := users.GetByID(ctx, exp.UserID())
	if err != nil {
		return nil, fmt.Errorf("exposure %s references missing user %s: %w (%v)", exp.ID(), exp.UserID(), domain.ErrDataConsistency, err)
	}
	equip, err := equipment.GetByID(ctx, exp.EquipmentID())
	if err != nil {
		return nil, fmt.Errorf("exposure %s references missing equipment %s: %w (%v)", exp.ID(), exp.EquipmentID(), domain.ErrDataConsistency, err)
	}
	return &ExposureReadModel{Exposure: exp, User: user, Equipment: equip}, nil
}
