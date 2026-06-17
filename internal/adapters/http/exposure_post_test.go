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

		reqBody := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q,"duration":30}`,
			seed.BobbyID, seed.AirCatID)
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		checkSpec(t, http.MethodPost, srv.URL+"/exposure", resp.StatusCode, resp.Header, body)

		var got map[string]any
		require.NoError(t, json.NewDecoder(bytes.NewReader(body)).Decode(&got))

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

		reqBody := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q,"duration":30}`,
			uuid.New(), seed.AirCatID)
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		checkSpec(t, http.MethodPost, srv.URL+"/exposure", resp.StatusCode, resp.Header, body)
		assertErrorSlug(t, body, "user_not_found")
	})

	t.Run("unknown equipment_id returns 404 equipment_not_found", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		reqBody := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q,"duration":30}`,
			seed.BobbyID, uuid.New())
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		checkSpec(t, http.MethodPost, srv.URL+"/exposure", resp.StatusCode, resp.Header, body)
		assertErrorSlug(t, body, "equipment_not_found")
	})

	t.Run("duration 0 returns 422 invalid_duration", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		reqBody := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q,"duration":0}`,
			seed.BobbyID, seed.AirCatID)
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
		checkSpec(t, http.MethodPost, srv.URL+"/exposure", resp.StatusCode, resp.Header, body)
		assertErrorSlug(t, body, "invalid_duration")
	})

	t.Run("malformed JSON returns 400 invalid_request", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString("{"))
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		checkSpec(t, http.MethodPost, srv.URL+"/exposure", resp.StatusCode, resp.Header, body)
		assertErrorSlug(t, body, "invalid_request")
	})

	t.Run("bad uuid string in user_id returns 400", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		reqBody := fmt.Sprintf(`{"user_id":"not-a-uuid","equipment_id":%q,"duration":30}`, seed.AirCatID)
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		checkSpec(t, http.MethodPost, srv.URL+"/exposure", resp.StatusCode, resp.Header, body)
	})

	t.Run("wrong-typed duration (json string) returns 400 invalid_request", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		// valid JSON, wrong type: duration as a string fails to decode into *int.
		reqBody := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q,"duration":"30"}`,
			seed.BobbyID, seed.AirCatID)
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		checkSpec(t, http.MethodPost, srv.URL+"/exposure", resp.StatusCode, resp.Header, body)
		assertErrorSlug(t, body, "invalid_request")
	})

	t.Run("trailing tokens after valid json returns 400 invalid_request", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		// A valid object followed by a second JSON token must be rejected, not recorded.
		reqBody := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q,"duration":30}{"x":1}`,
			seed.BobbyID, seed.AirCatID)
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		checkSpec(t, http.MethodPost, srv.URL+"/exposure", resp.StatusCode, resp.Header, body)
		assertErrorSlug(t, body, "invalid_request")
	})

	t.Run("missing duration field returns 400 invalid_request", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		reqBody := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q}`, seed.BobbyID, seed.AirCatID)
		resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()
		body := readBody(t, resp)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		checkSpec(t, http.MethodPost, srv.URL+"/exposure", resp.StatusCode, resp.Header, body)
		assertErrorSlug(t, body, "invalid_request")
	})
}

// assertErrorSlug decodes an error slug from raw body bytes.
func assertErrorSlug(t *testing.T, body []byte, wantSlug string) {
	t.Helper()
	var got map[string]any
	require.NoError(t, json.NewDecoder(bytes.NewReader(body)).Decode(&got))
	assert.Equal(t, wantSlug, got["error"])
}
