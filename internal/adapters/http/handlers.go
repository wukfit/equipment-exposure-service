package http

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/adapters/http/api"
	"github.com/wukfit/equipment-exposure-service/internal/app"
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
	rms, err := s.deps.ListExposures.Handle(r.Context())
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	out := make([]api.Exposure, 0, len(rms))
	for _, rm := range rms {
		out = append(out, toAPIExposure(rm))
	}
	writeJSON(w, http.StatusOK, out)
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
	start, end := normaliseUTC(params.StartingAt), normaliseUTC(params.EndingAt)
	from, to, err := app.ResolveWindow(s.deps.Clock, start, end)
	if err != nil {
		writeError(w, s.deps.Logger, err) // ErrInvalidWindow -> 400 invalid_window
		return
	}
	summary, err := s.deps.GetUserExposureSummary.Handle(r.Context(), userId, from, to)
	if err != nil {
		writeError(w, s.deps.Logger, err) // ErrUserNotFound -> 404
		return
	}
	writeJSON(w, http.StatusOK, toAPISummary(summary))
}

// normaliseUTC converts an optional timestamp to UTC (design §3).
func normaliseUTC(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
