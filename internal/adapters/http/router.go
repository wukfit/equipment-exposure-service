package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/wukfit/equipment-exposure-service/internal/adapters/http/api"
	"github.com/wukfit/equipment-exposure-service/internal/app"
	"github.com/wukfit/equipment-exposure-service/internal/app/command"
	"github.com/wukfit/equipment-exposure-service/internal/app/query"
)

// RouterDeps carries handler dependencies.
type RouterDeps struct {
	Logger                *slog.Logger
	Clock                 app.Clock
	RecordExposure        *command.RecordExposure
	GetExposure           *query.GetExposure
	ListExposures         *query.ListExposures
	GetUserExposureSummary *query.GetUserExposureSummary
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
	api.HandlerWithOptions(server, api.StdHTTPServerOptions{
		BaseRouter: mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			// Generated param-binding failures (malformed path uuid, bad/missing
			// query params) -> our JSON 400 error contract, not plain text. Keep
			// the specific parse error in the logs for debugging.
			deps.Logger.Debug("param binding failed", slog.String("error", err.Error()))
			writeError(w, deps.Logger, errBadRequest)
		},
	})

	var h http.Handler = mux
	h = recoverPanic(deps.Logger)(h)
	h = logging(deps.Logger)(h)
	h = requestID(h)
	return h
}
