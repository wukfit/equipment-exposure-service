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

func TestGetExposure(t *testing.T) {
	fixed := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := app.Clock(func() time.Time { return fixed })

	t.Run("round-trip: POST then GET returns 200 with matching fields", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		// POST to create
		body := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q,"duration":30}`,
			seed.BobbyID, seed.AirCatID)
		postResp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(body))
		require.NoError(t, err)
		defer postResp.Body.Close()
		require.Equal(t, http.StatusCreated, postResp.StatusCode)

		var postBody map[string]any
		require.NoError(t, json.NewDecoder(postResp.Body).Decode(&postBody))
		id, ok := postBody["id"].(string)
		require.True(t, ok, "POST response should contain id string")

		// GET by id
		getResp, err := http.Get(srv.URL + "/exposure/" + id)
		require.NoError(t, err)
		defer getResp.Body.Close()

		assert.Equal(t, http.StatusOK, getResp.StatusCode)

		var got map[string]any
		require.NoError(t, json.NewDecoder(getResp.Body).Decode(&got))

		assert.Equal(t, id, got["id"])
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

	t.Run("unknown id returns 404 exposure_not_found", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		getResp, err := http.Get(srv.URL + "/exposure/" + uuid.New().String())
		require.NoError(t, err)
		defer getResp.Body.Close()

		assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
		assertErrorSlug(t, getResp, "exposure_not_found")
	})

	t.Run("malformed path uuid returns 400 invalid_request as JSON", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		getResp, err := http.Get(srv.URL + "/exposure/not-a-uuid")
		require.NoError(t, err)
		defer getResp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, getResp.StatusCode)
		assertErrorSlug(t, getResp, "invalid_request")
	})
}
