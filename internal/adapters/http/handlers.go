package http

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/adapters/http/api"
	"github.com/wukfit/equipment-exposure-service/internal/app/command"
)

// Server implements api.ServerInterface.
type Server struct {
	deps RouterDeps
}

func (s *Server) RecordExposure(w http.ResponseWriter, r *http.Request) {
	var body api.RecordExposureJSONRequestBody
	if err := decodeJSONBody(r, &body); err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	// The spec marks the body required but not its individual fields, so oapi
	// generates them as pointers. A missing field is a clean 400, not a panic.
	if body.UserId == nil || body.EquipmentId == nil || body.Duration == nil {
		writeError(w, s.deps.Logger, errBadRequest)
		return
	}
	in := command.RecordExposureInput{
		UserID:      *body.UserId,
		EquipmentID: *body.EquipmentId,
		Duration:    *body.Duration,
	}
	rm, err := s.deps.RecordExposure.Handle(r.Context(), in)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAPIExposure(rm))
}

// --- stubs filled in by later slices ---

func (s *Server) GetExposures(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

func (s *Server) GetExposure(w http.ResponseWriter, r *http.Request, exposureId uuid.UUID) {
	rm, err := s.deps.GetExposure.Handle(r.Context(), exposureId)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toAPIExposure(rm))
}

func (s *Server) GetUserExposureSummary(w http.ResponseWriter, r *http.Request, userId uuid.UUID, params api.GetUserExposureSummaryParams) {
	notImplemented(w)
}

func notImplemented(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotImplemented, errorBody{Error: "not_implemented", Message: "endpoint not implemented yet"})
}
