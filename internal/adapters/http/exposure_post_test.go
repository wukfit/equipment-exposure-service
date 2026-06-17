package http_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wukfit/equipment-exposure-service/internal/app"
	"github.com/wukfit/equipment-exposure-service/internal/seed"
)

func TestPostExposure(t *testing.T) {
	fixed := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := app.Clock(func() time.Time { return fixed })

	t.Run("happy path returns 201 with correct fields", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		body := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q,"duration":30}`,
			seed.BobbyID, seed.AirCatID)
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var got map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))

		// id is a valid uuid
		idStr, _ := got["id"].(string)
		_, parseErr := uuid.Parse(idStr)
		assert.NoError(t, parseErr, "id should be a valid uuid")

		assert.InDelta(t, 0.525, got["a8"].(float64), 0.001)
		assert.InDelta(t, 4.0, got["points"].(float64), 0.001)
		assert.InDelta(t, 30.0, got["duration"].(float64), 0.001)

		user, _ := got["user"].(map[string]any)
		require.NotNil(t, user)
		assert.Equal(t, "Bobby Tables", user["name"])

		equipment, _ := got["equipment"].(map[string]any)
		require.NotNil(t, equipment)
		assert.Equal(t, "AirCat - Drill - 4337", equipment["name"])
		assert.InDelta(t, 2.1, equipment["vibration_magnitude"].(float64), 0.01)
	})

	t.Run("unknown user_id returns 404 user_not_found", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		body := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q,"duration":30}`,
			uuid.New(), seed.AirCatID)
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assertErrorSlug(t, resp, "user_not_found")
	})

	t.Run("unknown equipment_id returns 404 equipment_not_found", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		body := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q,"duration":30}`,
			seed.BobbyID, uuid.New())
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assertErrorSlug(t, resp, "equipment_not_found")
	})

	t.Run("duration 0 returns 422 invalid_duration", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		body := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q,"duration":0}`,
			seed.BobbyID, seed.AirCatID)
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
		assertErrorSlug(t, resp, "invalid_duration")
	})

	t.Run("malformed JSON returns 400 invalid_request", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString("{"))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assertErrorSlug(t, resp, "invalid_request")
	})

	t.Run("bad uuid string in user_id returns 400", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		body := fmt.Sprintf(`{"user_id":"not-a-uuid","equipment_id":%q,"duration":30}`, seed.AirCatID)
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("wrong-typed duration (json string) returns 400 invalid_request", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		// valid JSON, wrong type: duration as a string fails to decode into *int.
		body := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q,"duration":"30"}`,
			seed.BobbyID, seed.AirCatID)
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assertErrorSlug(t, resp, "invalid_request")
	})

	t.Run("missing duration field returns 400 invalid_request", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		body := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q}`, seed.BobbyID, seed.AirCatID)
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assertErrorSlug(t, resp, "invalid_request")
	})
}

func assertErrorSlug(t *testing.T, resp *http.Response, wantSlug string) {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, wantSlug, body["error"])
}
