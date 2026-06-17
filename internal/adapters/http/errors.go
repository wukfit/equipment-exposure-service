package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

var errBadRequest = errors.New("bad request")

// writeError maps a sentinel/domain error to an HTTP status + JSON body.
// Explicit default → 500 (no nil-deref, no per-handler duplication).
func writeError(w http.ResponseWriter, logger *slog.Logger, err error) {
	status, slug := http.StatusInternalServerError, "server_error"
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		status, slug = http.StatusNotFound, "user_not_found"
	case errors.Is(err, domain.ErrEquipmentNotFound):
		status, slug = http.StatusNotFound, "equipment_not_found"
	case errors.Is(err, domain.ErrExposureNotFound):
		status, slug = http.StatusNotFound, "exposure_not_found"
	case errors.Is(err, domain.ErrInvalidDuration):
		status, slug = http.StatusUnprocessableEntity, "invalid_duration"
	case errors.Is(err, errBadRequest):
		status, slug = http.StatusBadRequest, "invalid_request"
	}
	if status == http.StatusInternalServerError {
		logger.Error("request failed", slog.String("error", err.Error()))
	}
	writeJSON(w, status, errorBody{Error: slug, Message: err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
