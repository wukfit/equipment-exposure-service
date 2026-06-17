package httpapi_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/require"
)

var (
	specOnce   sync.Once
	specRouter routers.Router
)

func specRouterOnce(t *testing.T) routers.Router {
	t.Helper()
	specOnce.Do(func() {
		// panic (not require) inside Once: a missing/broken spec is a setup fault,
		// and panicking avoids the captured-t / cross-goroutine FailNow hazard.
		loader := openapi3.NewLoader()
		doc, err := loader.LoadFromFile("../../../spec.yaml")
		if err != nil {
			panic("openapi_test: load spec.yaml: " + err.Error())
		}
		if err := doc.Validate(loader.Context); err != nil {
			panic("openapi_test: validate spec.yaml: " + err.Error())
		}
		r, err := gorillamux.NewRouter(doc)
		if err != nil {
			panic("openapi_test: build router: " + err.Error())
		}
		specRouter = r
	})
	return specRouter
}

// validateAgainstSpec validates a captured response (status + body) against
// spec.yaml for the operation matching method+rawurl, returning the validation
// error (nil if valid). rawurl may be a full URL (host ignored).
func validateAgainstSpec(t *testing.T, method, rawurl string, status int, header http.Header, body []byte) error {
	t.Helper()
	router := specRouterOnce(t)
	req := httptest.NewRequest(method, rawurl, nil)
	route, pathParams, err := router.FindRoute(req)
	require.NoError(t, err, "no spec route for %s %s", method, rawurl)
	hdr := header.Clone()
	if hdr.Get("Content-Type") == "" {
		hdr.Set("Content-Type", "application/json")
	}
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: pathParams, Route: route},
		Status:                 status,
		Header:                 hdr,
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	}
	in.SetBodyBytes(body)
	return openapi3filter.ValidateResponse(context.Background(), in)
}

// checkSpec asserts a captured response conforms to spec.yaml (fails the test otherwise).
func checkSpec(t *testing.T, method, rawurl string, status int, header http.Header, body []byte) {
	t.Helper()
	require.NoError(t, validateAgainstSpec(t, method, rawurl, status, header, body),
		"response for %s %s (status %d) violates spec.yaml", method, rawurl, status)
}

// readBody reads the full response body so it can be both spec-validated and decoded.
func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return b
}

// TestSpecValidationHasTeeth proves the harness actually enforces the contract:
// now that the response/error schemas declare required fields, an incomplete
// body must FAIL validation (otherwise checkSpec would be a no-op safety net).
func TestSpecValidationHasTeeth(t *testing.T) {
	hdr := http.Header{"Content-Type": []string{"application/json"}}
	const summaryURL = "http://x/users/713be58e-0d79-4df2-a85c-9f44ca513a7d/exposure-summary"
	const exposureURL = "http://x/exposure/e8f7b50c-cc18-42f9-a275-0b4ead73f806"

	t.Run("empty 200 summary body is rejected (a8/points/user required)", func(t *testing.T) {
		require.Error(t, validateAgainstSpec(t, http.MethodGet, summaryURL, 200, hdr, []byte(`{}`)))
	})
	t.Run("summary missing user is rejected", func(t *testing.T) {
		require.Error(t, validateAgainstSpec(t, http.MethodGet, summaryURL, 200, hdr, []byte(`{"a8":1.0,"points":2}`)))
	})
	t.Run("empty 404 error body is rejected (error/message required)", func(t *testing.T) {
		require.Error(t, validateAgainstSpec(t, http.MethodGet, exposureURL, 404, hdr, []byte(`{}`)))
	})
	t.Run("complete summary body still passes", func(t *testing.T) {
		good := []byte(`{"a8":1.0,"points":2,"user":{"id":"713be58e-0d79-4df2-a85c-9f44ca513a7d","name":"Bobby"}}`)
		require.NoError(t, validateAgainstSpec(t, http.MethodGet, summaryURL, 200, hdr, good))
	})
}
