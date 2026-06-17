package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// decodeJSONBody decodes exactly one JSON value from the request body into v and
// rejects trailing data: anything after the first value (a second token or junk)
// makes the request malformed. Any failure maps to errBadRequest (→ 400).
func decodeJSONBody(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return errBadRequest
	}
	// A well-formed body holds a single JSON value; a second Decode must hit EOF.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errBadRequest
	}
	return nil
}
