package http_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wukfit/equipment-exposure-service/internal/app"
	"github.com/wukfit/equipment-exposure-service/internal/seed"
)

func TestGetExposures(t *testing.T) {
	fixed := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := app.Clock(func() time.Time { return fixed })

	t.Run("empty repo returns 200 with empty JSON array, not null", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		resp, err := http.Get(srv.URL + "/exposure")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var got []any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.NotNil(t, got, "body must be [] not null")
		assert.Len(t, got, 0)
	})

	t.Run("two exposures returns 200 with both present", func(t *testing.T) {
		srv, _ := newTestServer(t, clock)

		post := func(userID, equipID string, duration int) {
			body := fmt.Sprintf(`{"user_id":%q,"equipment_id":%q,"duration":%d}`, userID, equipID, duration)
			resp, err := http.Post(srv.URL+"/exposure", "application/json", bytes.NewBufferString(body))
			require.NoError(t, err)
			resp.Body.Close()
			require.Equal(t, http.StatusCreated, resp.StatusCode)
		}

		post(seed.BobbyID.String(), seed.AirCatID.String(), 30)
		post(seed.BobbyID.String(), seed.JCBID.String(), 120)

		resp, err := http.Get(srv.URL + "/exposure")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var got []map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		require.Len(t, got, 2)

		ids := map[string]bool{}
		for _, item := range got {
			id, ok := item["id"].(string)
			require.True(t, ok, "each item must have an id")
			ids[id] = true

			assert.NotEmpty(t, item["a8"])

			user, _ := item["user"].(map[string]any)
			require.NotNil(t, user)
			assert.NotEmpty(t, user["name"])

			equip, _ := item["equipment"].(map[string]any)
			require.NotNil(t, equip)
			assert.NotEmpty(t, equip["name"])
		}
		assert.Len(t, ids, 2, "both items must have distinct ids")
	})
}
