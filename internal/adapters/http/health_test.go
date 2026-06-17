package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	apphttp "github.com/wukfit/equipment-exposure-service/internal/adapters/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(apphttp.NewRouter(apphttp.RouterDeps{}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
