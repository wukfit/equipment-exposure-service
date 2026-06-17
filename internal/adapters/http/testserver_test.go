package http_test

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/wukfit/equipment-exposure-service/internal/adapters/events"
	apphttp "github.com/wukfit/equipment-exposure-service/internal/adapters/http"
	"github.com/wukfit/equipment-exposure-service/internal/adapters/repository/memory"
	"github.com/wukfit/equipment-exposure-service/internal/app"
	"github.com/wukfit/equipment-exposure-service/internal/app/command"
	"github.com/wukfit/equipment-exposure-service/internal/app/query"
	"github.com/wukfit/equipment-exposure-service/internal/seed"
)

// newTestServer wires the full app over in-memory repos seeded with the standard
// fixtures, using the supplied clock. It returns the running server plus the
// exposure repo handle so later slices can seed exposures at exact timestamps.
func newTestServer(t *testing.T, clock app.Clock) (*httptest.Server, *memory.ExposureRepo) {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	exposures := memory.NewExposureRepo()
	users := memory.NewUserRepo(seed.Users()...)
	equipment := memory.NewEquipmentRepo(seed.Equipment()...)
	deps := apphttp.RouterDeps{
		Logger: logger,
		RecordExposure: command.NewRecordExposure(
			exposures,
			users,
			equipment,
			events.NewSlogPublisher(logger),
			clock,
		),
		GetExposure:   query.NewGetExposure(exposures, users, equipment),
		ListExposures: query.NewListExposures(exposures, users, equipment),
	}
	srv := httptest.NewServer(apphttp.NewRouter(deps))
	t.Cleanup(srv.Close)
	return srv, exposures
}
