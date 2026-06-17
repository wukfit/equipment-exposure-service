package http

import (
	"github.com/wukfit/equipment-exposure-service/internal/adapters/http/api"
	"github.com/wukfit/equipment-exposure-service/internal/app/query"
)

func ptr[T any](v T) *T { return &v }

// toAPISummary builds the ExposureSummary API response from the query result.
func toAPISummary(s *query.UserExposureSummary) api.ExposureSummary {
	return api.ExposureSummary{
		A8:     ptr(float32(s.Partial.A8())),
		Points: ptr(float32(s.Partial.Points())),
		User:   ptr(api.User{Id: ptr(s.User.ID()), Name: ptr(s.User.Name())}),
	}
}

// toAPIExposure builds the embedded API Exposure from the read model.
func toAPIExposure(rm *query.ExposureReadModel) api.Exposure {
	e, u, eq := rm.Exposure, rm.User, rm.Equipment
	return api.Exposure{
		Id:       ptr(e.ID()),
		Duration: ptr(e.Duration()),
		A8:       ptr(float32(e.Partial().A8())),
		Points:   ptr(float32(e.Partial().Points())),
		User:     ptr(api.User{Id: ptr(u.ID()), Name: ptr(u.Name())}),
		Equipment: ptr(api.EquipmentItem{
			Id: ptr(eq.ID()), Name: ptr(eq.Name()), VibrationMagnitude: ptr(float32(eq.VibrationMagnitude())),
		}),
	}
}
