package query

import "github.com/wukfit/equipment-exposure-service/internal/domain"

// ExposureReadModel is the composed read-side projection of an exposure with
// its associated user and equipment, used to build the embedded API response.
type ExposureReadModel struct {
	Exposure  *domain.Exposure
	User      *domain.User
	Equipment *domain.EquipmentItem
}
