package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/wukfit/equipment-exposure-service/internal/adapters/http/api"
	"github.com/wukfit/equipment-exposure-service/internal/app/command"
)

// RouterDeps carries handler dependencies.
type RouterDeps struct {
	Logger         *slog.Logger
	RecordExposure *command.RecordExposure
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

	server := &Server{deps: deps}
	api.HandlerFromMux(server, mux)

	var h http.Handler = mux
	h = recoverPanic(deps.Logger)(h)
	h = logging(deps.Logger)(h)
	h = requestID(h)
	return h
}
