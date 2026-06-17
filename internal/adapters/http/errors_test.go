package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

func TestWriteError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantSlug   string
	}{
		{"user not found", domain.ErrUserNotFound, http.StatusNotFound, "user_not_found"},
		{"equipment not found", domain.ErrEquipmentNotFound, http.StatusNotFound, "equipment_not_found"},
		{"exposure not found", domain.ErrExposureNotFound, http.StatusNotFound, "exposure_not_found"},
		{"invalid duration", domain.ErrInvalidDuration, http.StatusUnprocessableEntity, "invalid_duration"},
		{"bad request", errBadRequest, http.StatusBadRequest, "invalid_request"},
		{"unknown error", errors.New("something unexpected"), http.StatusInternalServerError, "server_error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeError(w, logger, tc.err)

			resp := w.Result()
			assert.Equal(t, tc.wantStatus, resp.StatusCode)

			var body errorBody
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			assert.Equal(t, tc.wantSlug, body.Error)
		})
	}
}
