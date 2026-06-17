package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wukfit/equipment-exposure-service/internal/app"
	"github.com/wukfit/equipment-exposure-service/internal/domain"
	"github.com/wukfit/equipment-exposure-service/internal/seed"
)

func TestGetUserExposureSummary(t *testing.T) {
	// base is the "now" for the fixed test clock.
	base := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	clock := app.Clock(func() time.Time { return base })

	// seedBobby seeds Bobby's two canonical exposures inside the given window
	// [base-24h, base) using exact recordedAt timestamps.
	seedBobby := func(t *testing.T, exposures interface {
		Save(context.Context, *domain.Exposure) error
	}, at time.Time) {
		t.Helper()
		e1, err := domain.NewExposure(seed.BobbyID, seed.AirCatID, 30, 2.1, at)
		require.NoError(t, err)
		require.NoError(t, exposures.Save(context.Background(), e1))
		e2, err := domain.NewExposure(seed.BobbyID, seed.JCBID, 120, 4.0, at.Add(time.Minute))
		require.NoError(t, err)
		require.NoError(t, exposures.Save(context.Background(), e2))
	}

	summaryURL := func(srv *httptest.Server, userID uuid.UUID, query string) string {
		u := fmt.Sprintf("%s/users/%s/exposure-summary", srv.URL, userID)
		if query != "" {
			u += "?" + query
		}
		return u
	}

	t.Run("exact spec example timestamps accepted: two in-window exposures", func(t *testing.T) {
		srv, exposures := newTestServer(t, clock)

		// Seed inside [2025-01-01T00:00:00Z, 2025-01-31T23:59:59Z]
		seedBobby(t, exposures, time.Date(2025, 1, 10, 6, 0, 0, 0, time.UTC))

		url := summaryURL(srv, seed.BobbyID, "starting_at=2025-01-01T00:00:00Z&ending_at=2025-01-31T23:59:59Z")
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		checkSpec(t, http.MethodGet, url, resp.StatusCode, resp.Header, body)

		var got map[string]any
		require.NoError(t, json.NewDecoder(bytes.NewReader(body)).Decode(&got))
		assert.InDelta(t, 2.0678, got["a8"].(float64), 0.001)
		assert.Equal(t, float64(68), got["points"].(float64))
		user, _ := got["user"].(map[string]any)
		require.NotNil(t, user)
		assert.Equal(t, "Bobby Tables", user["name"])
	})

	t.Run("tz offset normalised: non-UTC offset still includes right exposures", func(t *testing.T) {
		srv, exposures := newTestServer(t, clock)

		// Seed at 2025-01-10T06:00:00Z
		seedBobby(t, exposures, time.Date(2025, 1, 10, 6, 0, 0, 0, time.UTC))

		// Express the same window with +01:00 offset (2025-01-01T01:00:00+01:00 == 2025-01-01T00:00:00Z)
		url := summaryURL(srv, seed.BobbyID, "starting_at=2025-01-01T01:00:00%2B01:00&ending_at=2025-02-01T00:00:00Z")
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		checkSpec(t, http.MethodGet, url, resp.StatusCode, resp.Header, body)

		var got map[string]any
		require.NoError(t, json.NewDecoder(bytes.NewReader(body)).Decode(&got))
		assert.InDelta(t, 2.0678, got["a8"].(float64), 0.001)
	})

	t.Run("half-open boundary: exposure AT ending_at excluded; AT starting_at included", func(t *testing.T) {
		srv, exposures := newTestServer(t, clock)

		windowStart := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)
		windowEnd := time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)

		// Exactly at start -> included
		atStart, err := domain.NewExposure(seed.BobbyID, seed.AirCatID, 30, 2.1, windowStart)
		require.NoError(t, err)
		require.NoError(t, exposures.Save(context.Background(), atStart))

		// Exactly at end -> excluded (half-open)
		atEnd, err := domain.NewExposure(seed.BobbyID, seed.JCBID, 120, 4.0, windowEnd)
		require.NoError(t, err)
		require.NoError(t, exposures.Save(context.Background(), atEnd))

		url := summaryURL(srv, seed.BobbyID, "starting_at=2025-01-10T00:00:00Z&ending_at=2025-01-20T00:00:00Z")
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		checkSpec(t, http.MethodGet, url, resp.StatusCode, resp.Header, body)

		// Only the at-start exposure counts (AirCat, 30 min @ 2.1)
		var got map[string]any
		require.NoError(t, json.NewDecoder(bytes.NewReader(body)).Decode(&got))
		assert.InDelta(t, 0.525, got["a8"].(float64), 0.001)
		assert.Equal(t, float64(4), got["points"].(float64))
	})

	t.Run("user isolation: Alice exposures do not affect Bobby's summary", func(t *testing.T) {
		srv, exposures := newTestServer(t, clock)

		// Seed Bobby's two exposures
		seedBobby(t, exposures, time.Date(2025, 1, 10, 6, 0, 0, 0, time.UTC))

		// Alice: large exposure in same window
		alice, err := domain.NewExposure(seed.AliceID, seed.AirCatID, 480, 2.1, time.Date(2025, 1, 10, 6, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		require.NoError(t, exposures.Save(context.Background(), alice))

		url := summaryURL(srv, seed.BobbyID, "starting_at=2025-01-01T00:00:00Z&ending_at=2025-01-31T23:59:59Z")
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		checkSpec(t, http.MethodGet, url, resp.StatusCode, resp.Header, body)

		var got map[string]any
		require.NoError(t, json.NewDecoder(bytes.NewReader(body)).Decode(&got))
		// Bobby's summary unchanged despite Alice's large exposure
		assert.InDelta(t, 2.0678, got["a8"].(float64), 0.001)
		assert.Equal(t, float64(68), got["points"].(float64))
	})

	t.Run("empty window: user with no in-window exposures returns 200 zeros", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		url := summaryURL(srv, seed.BobbyID, "starting_at=2025-01-01T00:00:00Z&ending_at=2025-01-31T23:59:59Z")
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		checkSpec(t, http.MethodGet, url, resp.StatusCode, resp.Header, body)

		var got map[string]any
		require.NoError(t, json.NewDecoder(bytes.NewReader(body)).Decode(&got))
		assert.Equal(t, float64(0), got["a8"].(float64))
		assert.Equal(t, float64(0), got["points"].(float64))
	})

	t.Run("default window: no params uses [now-24h, now) from fixed clock", func(t *testing.T) {
		// now = base (2025-01-15T12:00:00Z); default window [base-24h, base)
		srv, exposures := newTestServer(t, clock)

		// Seed 1 hour before "now" -> inside default window
		inWindow := base.Add(-time.Hour)
		e, err := domain.NewExposure(seed.BobbyID, seed.AirCatID, 30, 2.1, inWindow)
		require.NoError(t, err)
		require.NoError(t, exposures.Save(context.Background(), e))

		url := summaryURL(srv, seed.BobbyID, "")
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		checkSpec(t, http.MethodGet, url, resp.StatusCode, resp.Header, body)

		var got map[string]any
		require.NoError(t, json.NewDecoder(bytes.NewReader(body)).Decode(&got))
		// One AirCat exposure for 30 min @ 2.1 m/s²
		assert.InDelta(t, 0.525, got["a8"].(float64), 0.001)
		assert.Greater(t, got["a8"].(float64), 0.0)
	})

	t.Run("start > end returns 400 invalid_window", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		url := summaryURL(srv, seed.BobbyID, "starting_at=2025-02-01T00:00:00Z&ending_at=2025-01-01T00:00:00Z")
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		checkSpec(t, http.MethodGet, url, resp.StatusCode, resp.Header, body)
		assertErrorSlug(t, body, "invalid_window")
	})

	t.Run("unknown user returns 404 user_not_found", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		unknown := uuid.MustParse("00000000-0000-0000-0000-000000000000")
		url := summaryURL(srv, unknown, "starting_at=2025-01-01T00:00:00Z&ending_at=2025-01-31T23:59:59Z")
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		checkSpec(t, http.MethodGet, url, resp.StatusCode, resp.Header, body)
		assertErrorSlug(t, body, "user_not_found")
	})

	t.Run("malformed date param returns 400 invalid_request", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		url := summaryURL(srv, seed.BobbyID, "starting_at=notadate")
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		checkSpec(t, http.MethodGet, url, resp.StatusCode, resp.Header, body)
		assertErrorSlug(t, body, "invalid_request")
	})
}
