package http_test

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
		loader := openapi3.NewLoader()
		doc, err := loader.LoadFromFile("../../../spec.yaml")
		require.NoError(t, err)
		require.NoError(t, doc.Validate(loader.Context))
		r, err := gorillamux.NewRouter(doc)
		require.NoError(t, err)
		specRouter = r
	})
	return specRouter
}

// checkSpec validates a captured response (status + body) against spec.yaml for
// the operation matching method+rawurl. rawurl may be a full URL (host ignored).
func checkSpec(t *testing.T, method, rawurl string, status int, header http.Header, body []byte) {
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
	require.NoError(t, openapi3filter.ValidateResponse(context.Background(), in),
		"response for %s %s (status %d) violates spec.yaml", method, rawurl, status)
}

// readBody reads the full response body so it can be both spec-validated and decoded.
func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return b
}
