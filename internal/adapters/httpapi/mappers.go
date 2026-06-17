package httpapi

import (
	"github.com/wukfit/equipment-exposure-service/internal/adapters/httpapi/api"
	"github.com/wukfit/equipment-exposure-service/internal/app/query"
)

// toAPISummary builds the ExposureSummary API response from the query result.
// All response fields are required in the spec, so the generated types are
// values (not pointers) — every field is always populated.
func toAPISummary(s *query.UserExposureSummary) api.ExposureSummary {
	return api.ExposureSummary{
		A8:     float32(s.Partial.A8()),
		Points: float32(s.Partial.Points()),
		User:   api.User{Id: s.User.ID(), Name: s.User.Name()},
	}
}

// toAPIExposure builds the embedded API Exposure from the read model.
func toAPIExposure(rm *query.ExposureReadModel) api.Exposure {
	e, u, eq := rm.Exposure, rm.User, rm.Equipment
	return api.Exposure{
		Id:       e.ID(),
		Duration: e.Duration(),
		A8:       float32(e.Partial().A8()),
		Points:   float32(e.Partial().Points()),
		User:     api.User{Id: u.ID(), Name: u.Name()},
		Equipment: api.EquipmentItem{
			Id: eq.ID(), Name: eq.Name(), VibrationMagnitude: float32(eq.VibrationMagnitude()),
		},
	}
}
