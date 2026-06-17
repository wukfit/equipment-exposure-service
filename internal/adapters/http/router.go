package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// RouterDeps carries handler dependencies. Empty in slice 0; populated as slices land.
type RouterDeps struct {
	Logger *slog.Logger
	// Server ServerInterface  // wired from slice 3 onward
}

func NewRouter(deps RouterDeps) http.Handler {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	var h http.Handler = mux
	h = recoverPanic(deps.Logger)(h)
	h = logging(deps.Logger)(h)
	h = requestID(h)
	return h
}
