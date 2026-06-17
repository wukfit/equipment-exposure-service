# Equipment Exposure Service — Implementation Plan

> Implementation plan, built task-by-task with TDD. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the HAVS Equipment Exposure HTTP API in Go per `spec.yaml`, in 9 independently-shippable slices (one PR each).

**Architecture:** Ports & adapters / DDD. `domain` holds the `Exposure` aggregate, `User`/`EquipmentItem`, the `PartialExposure` value object (the HAVS maths + aggregation), repository ports, and an `EventPublisher` port. `app` holds one command (`RecordExposure`) and three queries. `adapters/http` (stdlib `net/http` + oapi-codegen) maps requests→commands/queries and read-models→responses. Persistence is in-memory behind a port (Postgres is a stretch slice).

**Tech Stack:** Go 1.25 · stdlib `net/http` (Go 1.22 `ServeMux`) · `github.com/oapi-codegen/oapi-codegen/v2` (types + std-http-server) · `github.com/google/uuid` · `github.com/stretchr/testify` · `github.com/getkin/kin-openapi` (response validation in tests) · `log/slog`.

**Module path:** `github.com/wukfit/equipment-exposure-service`

---

## Conventions (apply to every slice)

- **One PR per slice:** branch off `main`, implement with TDD commits, push, open the PR, merge, and return to `main` before the next slice. Never commit feature work to `main`.
- **TDD rhythm per component:** write failing test → run, confirm red → minimal implementation → run, confirm green → commit. Conventional commit messages (`feat:`, `test:`, `chore:`).
- **Every run uses `-race`:** `go test -race ./...`.

## File structure (built up across slices)

```
go.mod / go.sum
spec.yaml                                   (already present)
oapi-codegen.yaml                           codegen config (slice 0)
Dockerfile · docker-compose.yml · Makefile  (slice 0)
cmd/server/main.go                          wiring, config, graceful shutdown
internal/
  domain/
    partial_exposure.go     PartialExposure VO + NewPartialExposure + Aggregate   (slice 1)
    equipment.go            EquipmentItem + NewEquipmentItem                       (slice 2)
    user.go                 User + NewUser                                         (slice 2)
    exposure.go             Exposure aggregate + NewExposure/NewExposureFromStore  (slice 2)
    errors.go               sentinel errors                                        (slice 2)
    repository.go           ExposureRepository / EquipmentRepository / UserRepository ports (slice 2)
    events.go               ExposureRecorded event + EventPublisher port           (slice 3)
  app/
    clock.go                Clock func() time.Time                                 (slice 2)
    command/record_exposure.go                                                     (slice 3)
    query/get_exposure.go · list_exposures.go · get_user_exposure_summary.go       (slices 4,5,6)
    query/read_model.go     ExposureReadModel                                      (slice 4)
    window.go               window resolution (defaults)                           (slice 6)
  adapters/
    http/
      api/                  GENERATED (types.gen.go, server.gen.go) package `api`  (slice 0)
      router.go             mux wiring + middleware                                (slice 0)
      middleware.go         requestID, slog logging, recover                       (slice 0)
      errors.go             sentinel→status mapping helper                         (slice 3)
      mappers.go            domain/read-model → api types                          (slice 3)
      handlers.go           ServerInterface implementation                         (slices 3-6)
    repository/memory/
      exposure_repo.go · equipment_repo.go · user_repo.go                          (slice 2)
    events/slog_publisher.go                                                       (slice 3)
  seed/seed.go              fixed fixtures (shared app + tests)                    (slice 2)
internal/contracttest/repository.go  shared repository contract suite              (slice 2)
```

---

## Task 0: Walking skeleton (PR #1)

**Goal:** A booting `net/http` server with `/health`, generated API stubs, and one-command run — no domain logic.

**Files:**
- Create: `go.mod`, `oapi-codegen.yaml`, `cmd/server/main.go`, `internal/adapters/http/router.go`, `internal/adapters/http/middleware.go`, `internal/adapters/http/health_test.go`, `Dockerfile`, `docker-compose.yml`, `Makefile`

- [ ] **Step 1: Init module + dependencies**

```bash
go mod init github.com/wukfit/equipment-exposure-service
go get github.com/google/uuid@latest
go get github.com/oapi-codegen/runtime@latest
go get github.com/stretchr/testify@latest
go get github.com/getkin/kin-openapi@latest
go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
```
Ensure `go.mod` shows `go 1.25` (edit the `go` line if needed; add `toolchain go1.25.4`).

- [ ] **Step 2: Write codegen config** — `oapi-codegen.yaml`

```yaml
package: api
output: internal/adapters/http/api/api.gen.go
generate:
  models: true
  std-http-server: true
output-options:
  skip-prune: false
```

- [ ] **Step 3: Generate the API package**

```bash
go tool oapi-codegen -config oapi-codegen.yaml spec.yaml
```
Expected: `internal/adapters/http/api/api.gen.go` created with a `ServerInterface`, request/response model types (`Exposure`, `ExposureSummary`, `User`, `EquipmentItem`, the `recordExposure` body type), and a registration helper. **Open the file and note the exact generated names** (struct field names, the request-body type name, whether `number` fields are `*float32`/`*float64`, whether fields are pointers). Slices 3-6 reference these; adjust the mapper field assignments to the actual generated names/types. This is the only place generated-name specifics are resolved.

- [ ] **Step 4: Write the failing health test** — `internal/adapters/http/health_test.go`

```go
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
```

- [ ] **Step 5: Run, confirm red** — `go test ./internal/adapters/http/ -run TestHealth` → FAIL (undefined `NewRouter`).

- [ ] **Step 6: Implement middleware** — `internal/adapters/http/middleware.go`

```go
package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			id, _ := r.Context().Value(requestIDKey).(string)
			logger.Info("request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.status),
				slog.Int64("elapsed_ms", time.Since(start).Milliseconds()),
				slog.String("request_id", id),
			)
		})
	}
}

func recoverPanic(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic", slog.Any("recover", rec))
					http.Error(w, `{"error":"server_error","message":"internal error"}`, http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) { s.status = code; s.ResponseWriter.WriteHeader(code) }
```

- [ ] **Step 7: Implement the router** — `internal/adapters/http/router.go`

```go
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
```

- [ ] **Step 8: Run, confirm green** — `go test -race ./internal/adapters/http/ -run TestHealth` → PASS.

- [ ] **Step 9: Implement main** — `cmd/server/main.go`

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apphttp "github.com/wukfit/equipment-exposure-service/internal/adapters/http"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	router := apphttp.NewRouter(apphttp.RouterDeps{Logger: logger})
	srv := &http.Server{Addr: ":" + getenv("PORT", "8080"), Handler: router}

	go func() {
		logger.Info("starting server", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
	}
	logger.Info("server stopped")
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
```

- [ ] **Step 10: Dockerfile** — multi-stage, pinned, non-root

```dockerfile
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
```

- [ ] **Step 11: docker-compose.yml** (app only — in-memory persistence)

```yaml
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      PORT: "8080"
      LOG_LEVEL: "info"
```

- [ ] **Step 12: Makefile**

```makefile
.PHONY: run test generate docker
run:
	go run ./cmd/server
test:
	go test -race -cover ./...
generate:
	go tool oapi-codegen -config oapi-codegen.yaml spec.yaml
docker:
	docker compose up --build
```

- [ ] **Step 13: Verify build + boot + compose**

```bash
go build ./...
go test -race ./...
docker compose up --build -d && sleep 3 && curl -fsS localhost:8080/health && docker compose down
```
Expected: build clean; tests pass; curl prints `{"status":"ok"}`.

- [ ] **Step 14: Ship** — open the PR (branch `walking-skeleton`, PR against `main`). Includes the pending `.gitignore` update.

---

## Task 1: HAVS maths in isolation (PR #2)

**Goal:** `PartialExposure` value object with the corrected float calc + `Aggregate`. Pure domain, exhaustively tested. Highest correctness risk → done first.

**Files:**
- Create: `internal/domain/partial_exposure.go`, `internal/domain/partial_exposure_test.go`

- [ ] **Step 1: Failing table test** — `partial_exposure_test.go`

```go
package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPartialExposure(t *testing.T) {
	cases := []struct {
		name      string
		magnitude float64
		minutes   int
		wantA8    float64
		wantPts   float64
	}{
		{"aircat 30min", 2.1, 30, 0.525, 4},
		{"aircat 60min", 2.1, 60, 0.7425, 9},
		{"jcb 30min", 4.0, 30, 1.0, 16},
		{"jcb 2h", 4.0, 120, 2.0, 64},
		{"jcb 8h", 4.0, 480, 4.0, 256},
		{"zero duration", 2.1, 0, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := NewPartialExposure(c.magnitude, c.minutes)
			assert.InDelta(t, c.wantA8, p.A8(), 0.001)
			assert.Equal(t, c.wantPts, p.Points())
		})
	}
}

func TestAggregate(t *testing.T) {
	t.Run("empty is zero", func(t *testing.T) {
		got := Aggregate(nil)
		assert.Equal(t, 0.0, got.A8())
		assert.Equal(t, 0.0, got.Points())
	})
	t.Run("single is identity", func(t *testing.T) {
		p := NewPartialExposure(2.1, 30)
		got := Aggregate([]PartialExposure{p})
		assert.InDelta(t, p.A8(), got.A8(), 0.0001)
		assert.Equal(t, p.Points(), got.Points())
	})
	t.Run("rss for a8, sum for points", func(t *testing.T) {
		parts := []PartialExposure{NewPartialExposure(2.1, 30), NewPartialExposure(4.0, 120)}
		got := Aggregate(parts)
		assert.InDelta(t, 2.0678, got.A8(), 0.001) // sqrt(0.525^2 + 2.0^2)
		assert.Equal(t, 68.0, got.Points())        // 4 + 64
	})
}
```

- [ ] **Step 2: Run, confirm red** — `go test ./internal/domain/ -run 'PartialExposure|Aggregate'` → FAIL (undefined).

- [ ] **Step 3: Implement** — `partial_exposure.go`

```go
package domain

import "math"

// PartialExposure holds the two HAVS partial-exposure metrics for an activity.
type PartialExposure struct {
	a8     float64
	points float64
}

// NewPartialExposure computes the HSE partial exposure from a vibration
// magnitude (m/s^2) and a duration in minutes.
//
// NOTE: the brief's reference code used Go integer division on (minutes/60)/8,
// which truncates to 0 for any exposure under 8 hours. This uses float division
// (the mathematically intended form). See docs/design.md §3.
func NewPartialExposure(magnitude float64, minutes int) PartialExposure {
	hours := float64(minutes) / 60.0
	return PartialExposure{
		a8:     magnitude * math.Sqrt(hours/8.0),
		points: math.Round(math.Pow(magnitude/2.5, 2) * (hours / 8.0) * 100),
	}
}

func (p PartialExposure) A8() float64     { return p.a8 }
func (p PartialExposure) Points() float64 { return p.points }

// Aggregate combines partial exposures: A(8) by root-sum-of-squares (energy
// combination, per HSE), points by linear sum. See docs/design.md §5.4.
func Aggregate(parts []PartialExposure) PartialExposure {
	var sumSquares, sumPoints float64
	for _, p := range parts {
		sumSquares += p.a8 * p.a8
		sumPoints += p.points
	}
	return PartialExposure{a8: math.Sqrt(sumSquares), points: sumPoints}
}
```

- [ ] **Step 4: Run, confirm green** — `go test -race -cover ./internal/domain/` → PASS.

- [ ] **Step 5: Commit + ship** — `feat: add HAVS partial-exposure calculation and aggregation`; open the PR (branch `havs-calculation`).

---

## Task 2: Entities, ports, seed, in-memory repo (PR #3)

**Goal:** Domain entities + repository ports + in-memory adapters + seed fixtures + shared repository contract test.

**Files:**
- Create: `internal/domain/{equipment.go,user.go,exposure.go,errors.go,repository.go}`, `internal/app/clock.go`, `internal/adapters/repository/memory/{exposure_repo.go,equipment_repo.go,user_repo.go}`, `internal/seed/seed.go`, `internal/contracttest/repository.go`, and matching `_test.go` files.

- [ ] **Step 1: Sentinel errors** — `internal/domain/errors.go`

```go
package domain

import "errors"

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrEquipmentNotFound = errors.New("equipment not found")
	ErrExposureNotFound  = errors.New("exposure not found")
	ErrInvalidDuration   = errors.New("invalid duration")
	ErrInvalidMagnitude  = errors.New("invalid magnitude")
	ErrInvalidName       = errors.New("invalid name")
)
```

- [ ] **Step 2: EquipmentItem + User (test-first)** — `equipment_test.go`, `user_test.go`

```go
// equipment_test.go
package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEquipmentItem(t *testing.T) {
	id := uuid.New()
	e, err := NewEquipmentItem(id, "AirCat - Drill - 4337", 2.1)
	require.NoError(t, err)
	assert.Equal(t, id, e.ID())
	assert.Equal(t, 2.1, e.VibrationMagnitude())

	_, err = NewEquipmentItem(id, "", 2.1)
	assert.ErrorIs(t, err, ErrInvalidName)
	_, err = NewEquipmentItem(id, "x", 0)
	assert.ErrorIs(t, err, ErrInvalidMagnitude)
}
```
```go
// user_test.go
package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUser(t *testing.T) {
	id := uuid.New()
	u, err := NewUser(id, "Bobby Tables")
	require.NoError(t, err)
	assert.Equal(t, "Bobby Tables", u.Name())

	_, err = NewUser(id, "")
	assert.ErrorIs(t, err, ErrInvalidName)
}
```

- [ ] **Step 3: Run red, then implement** — `equipment.go`, `user.go`

```go
// equipment.go
package domain

import "github.com/google/uuid"

type EquipmentItem struct {
	id                 uuid.UUID
	name               string
	vibrationMagnitude float64
}

func NewEquipmentItem(id uuid.UUID, name string, magnitude float64) (*EquipmentItem, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	if magnitude <= 0 {
		return nil, ErrInvalidMagnitude
	}
	return &EquipmentItem{id: id, name: name, vibrationMagnitude: magnitude}, nil
}

func (e *EquipmentItem) ID() uuid.UUID            { return e.id }
func (e *EquipmentItem) Name() string             { return e.name }
func (e *EquipmentItem) VibrationMagnitude() float64 { return e.vibrationMagnitude }
```
```go
// user.go
package domain

import "github.com/google/uuid"

type User struct {
	id   uuid.UUID
	name string
}

func NewUser(id uuid.UUID, name string) (*User, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	return &User{id: id, name: name}, nil
}

func (u *User) ID() uuid.UUID { return u.id }
func (u *User) Name() string  { return u.name }
```

- [ ] **Step 4: Exposure aggregate (test-first)** — `exposure_test.go`

```go
package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExposure(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	e, err := NewExposure(uuid.New(), uuid.New(), 30, 2.1, now)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, e.ID())
	assert.Equal(t, 30, e.Duration())
	assert.InDelta(t, 0.525, e.Partial().A8(), 0.001)
	assert.Equal(t, now, e.RecordedAt())

	_, err = NewExposure(uuid.New(), uuid.New(), 0, 2.1, now)
	assert.ErrorIs(t, err, ErrInvalidDuration)
}
```

- [ ] **Step 5: Run red, then implement** — `exposure.go`

```go
package domain

import (
	"time"

	"github.com/google/uuid"
)

type Exposure struct {
	id          uuid.UUID
	userID      uuid.UUID
	equipmentID uuid.UUID
	duration    int
	partial     PartialExposure
	recordedAt  time.Time
}

// NewExposure creates a new exposure, computing its partial exposure from the
// equipment's vibration magnitude and the duration.
func NewExposure(userID, equipmentID uuid.UUID, minutes int, magnitude float64, recordedAt time.Time) (*Exposure, error) {
	if minutes <= 0 {
		return nil, ErrInvalidDuration
	}
	return &Exposure{
		id:          uuid.New(),
		userID:      userID,
		equipmentID: equipmentID,
		duration:    minutes,
		partial:     NewPartialExposure(magnitude, minutes),
		recordedAt:  recordedAt,
	}, nil
}

// NewExposureFromStore reconstitutes an exposure from persisted state without
// recomputing (used by repository adapters).
func NewExposureFromStore(id, userID, equipmentID uuid.UUID, minutes int, partial PartialExposure, recordedAt time.Time) *Exposure {
	return &Exposure{id: id, userID: userID, equipmentID: equipmentID, duration: minutes, partial: partial, recordedAt: recordedAt}
}

func (e *Exposure) ID() uuid.UUID          { return e.id }
func (e *Exposure) UserID() uuid.UUID      { return e.userID }
func (e *Exposure) EquipmentID() uuid.UUID { return e.equipmentID }
func (e *Exposure) Duration() int          { return e.duration }
func (e *Exposure) Partial() PartialExposure { return e.partial }
func (e *Exposure) RecordedAt() time.Time  { return e.recordedAt }
```

- [ ] **Step 6: Repository ports** — `internal/domain/repository.go`

```go
package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ExposureRepository interface {
	Save(ctx context.Context, e *Exposure) error
	GetByID(ctx context.Context, id uuid.UUID) (*Exposure, error)
	List(ctx context.Context) ([]*Exposure, error)
	ListByUserInWindow(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]*Exposure, error)
}

type EquipmentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*EquipmentItem, error)
}

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
}
```

- [ ] **Step 7: Clock** — `internal/app/clock.go`

```go
package app

import "time"

// Clock returns the current time; injected for deterministic tests.
type Clock func() time.Time

// SystemClock returns wall-clock UTC.
func SystemClock() time.Time { return time.Now().UTC() }
```

- [ ] **Step 8: Shared repository contract test** — `internal/contracttest/repository.go`

```go
package contracttest

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

// RunExposureRepository exercises any ExposureRepository implementation.
func RunExposureRepository(t *testing.T, newRepo func() domain.ExposureRepository) {
	ctx := context.Background()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("save then get", func(t *testing.T) {
		repo := newRepo()
		e, _ := domain.NewExposure(uuid.New(), uuid.New(), 30, 2.1, base)
		require.NoError(t, repo.Save(ctx, e))
		got, err := repo.GetByID(ctx, e.ID())
		require.NoError(t, err)
		assert.Equal(t, e.ID(), got.ID())
	})

	t.Run("get missing returns ErrExposureNotFound", func(t *testing.T) {
		repo := newRepo()
		_, err := repo.GetByID(ctx, uuid.New())
		assert.ErrorIs(t, err, domain.ErrExposureNotFound)
	})

	t.Run("window filters by user and time", func(t *testing.T) {
		repo := newRepo()
		user := uuid.New()
		other := uuid.New()
		inWindow, _ := domain.NewExposure(user, uuid.New(), 30, 2.1, base)
		before, _ := domain.NewExposure(user, uuid.New(), 30, 2.1, base.Add(-48*time.Hour))
		otherUser, _ := domain.NewExposure(other, uuid.New(), 30, 2.1, base)
		for _, e := range []*domain.Exposure{inWindow, before, otherUser} {
			require.NoError(t, repo.Save(ctx, e))
		}
		got, err := repo.ListByUserInWindow(ctx, user, base.Add(-24*time.Hour), base.Add(time.Hour))
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, inWindow.ID(), got[0].ID())
	})

	t.Run("concurrent saves are safe", func(t *testing.T) {
		repo := newRepo()
		done := make(chan struct{})
		for i := 0; i < 50; i++ {
			go func() {
				e, _ := domain.NewExposure(uuid.New(), uuid.New(), 30, 2.1, base)
				_ = repo.Save(ctx, e)
				done <- struct{}{}
			}()
		}
		for i := 0; i < 50; i++ {
			<-done
		}
		all, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Len(t, all, 50)
	})
}
```

- [ ] **Step 9: In-memory adapters** — `internal/adapters/repository/memory/exposure_repo.go` (+ equipment_repo.go, user_repo.go)

```go
// exposure_repo.go
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type ExposureRepo struct {
	mu   sync.RWMutex
	data map[uuid.UUID]*domain.Exposure
}

func NewExposureRepo() *ExposureRepo {
	return &ExposureRepo{data: make(map[uuid.UUID]*domain.Exposure)}
}

func (r *ExposureRepo) Save(_ context.Context, e *domain.Exposure) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[e.ID()] = e
	return nil
}

func (r *ExposureRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Exposure, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.data[id]
	if !ok {
		return nil, domain.ErrExposureNotFound
	}
	return e, nil
}

func (r *ExposureRepo) List(_ context.Context) ([]*domain.Exposure, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*domain.Exposure, 0, len(r.data))
	for _, e := range r.data {
		out = append(out, e)
	}
	sortByRecordedAt(out)
	return out, nil
}

func (r *ExposureRepo) ListByUserInWindow(_ context.Context, userID uuid.UUID, start, end time.Time) ([]*domain.Exposure, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*domain.Exposure
	for _, e := range r.data {
		if e.UserID() != userID {
			continue
		}
		// inclusive window [start, end]
		if e.RecordedAt().Before(start) || e.RecordedAt().After(end) {
			continue
		}
		out = append(out, e)
	}
	sortByRecordedAt(out)
	return out, nil
}
```
Add a small `sortByRecordedAt` helper (stable order by `RecordedAt` then `ID`) in the same package, and trivial `EquipmentRepo`/`UserRepo` (map + `GetByID` → `ErrEquipmentNotFound`/`ErrUserNotFound`). Wire the contract test: `memory_test.go` calls `contracttest.RunExposureRepository(t, func() domain.ExposureRepository { return NewExposureRepo() })`.

- [ ] **Step 10: Seed fixtures** — `internal/seed/seed.go`

```go
package seed

import (
	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

var (
	AirCatID = uuid.MustParse("2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49")
	JCBID    = uuid.MustParse("36603447-2f30-41b1-a908-526c0b6f1755")
	BobbyID  = uuid.MustParse("713be58e-0d79-4df2-a85c-9f44ca513a7d")
	AliceID  = uuid.MustParse("a52a3c1e-7b2d-4c9a-9f0e-1d6b8c4f2a10")
)

func Equipment() []*domain.EquipmentItem {
	a, _ := domain.NewEquipmentItem(AirCatID, "AirCat - Drill - 4337", 2.1)
	j, _ := domain.NewEquipmentItem(JCBID, "JCB - Hydraulic Breaker - CEJCBHM25", 4.0)
	return []*domain.EquipmentItem{a, j}
}

func Users() []*domain.User {
	b, _ := domain.NewUser(BobbyID, "Bobby Tables")
	al, _ := domain.NewUser(AliceID, "Alice Stone")
	return []*domain.User{b, al}
}
```
Add a `seed_test.go` asserting counts/IDs, and equipment/user repo constructors that accept a slice to preload (e.g. `memory.NewEquipmentRepo(seed.Equipment()...)`).

- [ ] **Step 11: Run all** — `go test -race -cover ./...` → PASS.

- [ ] **Step 12: Ship** — `feat: add domain entities, ports, in-memory repos, seed`; open the PR (branch `domain-and-repos`).

---

## Task 3: POST /exposure — first full vertical (PR #4)

**Goal:** `RecordExposure` command, HTTP handler, request/response mapping, error-mapping helper, and the `EventPublisher` seam. Outside-in: blackbox test first.

**Files:**
- Create: `internal/domain/events.go`, `internal/app/command/record_exposure.go`, `internal/adapters/events/slog_publisher.go`, `internal/adapters/http/{errors.go,mappers.go,handlers.go}`, `internal/app/query/read_model.go`, and tests incl. `internal/adapters/http/exposure_post_test.go` (blackbox), plus a `testserver` helper.

- [ ] **Step 1: EventPublisher port + event** — `internal/domain/events.go`

```go
package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ExposureRecorded struct {
	ExposureID  uuid.UUID
	UserID      uuid.UUID
	EquipmentID uuid.UUID
	A8          float64
	Points      float64
	RecordedAt  time.Time
}

type EventPublisher interface {
	Publish(ctx context.Context, event ExposureRecorded) error
}
```

- [ ] **Step 2: Read model** — `internal/app/query/read_model.go`

```go
package query

import "github.com/wukfit/equipment-exposure-service/internal/domain"

// ExposureReadModel is the composed read-side projection of an exposure with
// its associated user and equipment, used to build the embedded API response.
type ExposureReadModel struct {
	Exposure  *domain.Exposure
	User      *domain.User
	Equipment *domain.EquipmentItem
}
```

- [ ] **Step 3: RecordExposure command (test-first)** — `record_exposure_test.go`

```go
package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wukfit/equipment-exposure-service/internal/app"
	"github.com/wukfit/equipment-exposure-service/internal/app/command"
	"github.com/wukfit/equipment-exposure-service/internal/domain"
	"github.com/wukfit/equipment-exposure-service/internal/adapters/repository/memory"
	"github.com/wukfit/equipment-exposure-service/internal/seed"
)

type spyPublisher struct{ events []domain.ExposureRecorded }

func (s *spyPublisher) Publish(_ context.Context, e domain.ExposureRecorded) error {
	s.events = append(s.events, e)
	return nil
}

func TestRecordExposure(t *testing.T) {
	fixed := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := app.Clock(func() time.Time { return fixed })
	pub := &spyPublisher{}
	h := command.NewRecordExposure(
		memory.NewExposureRepo(),
		memory.NewUserRepo(seed.Users()...),
		memory.NewEquipmentRepo(seed.Equipment()...),
		pub, clock,
	)

	rm, err := h.Handle(context.Background(), command.RecordExposureInput{
		UserID: seed.BobbyID, EquipmentID: seed.AirCatID, Duration: 30,
	})
	require.NoError(t, err)
	assert.InDelta(t, 0.525, rm.Exposure.Partial().A8(), 0.001)
	assert.Equal(t, fixed, rm.Exposure.RecordedAt())
	require.Len(t, pub.events, 1)
	assert.Equal(t, rm.Exposure.ID(), pub.events[0].ExposureID)

	_, err = h.Handle(context.Background(), command.RecordExposureInput{
		UserID: uuid.New(), EquipmentID: seed.AirCatID, Duration: 30,
	})
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}
```

- [ ] **Step 4: Run red, then implement** — `record_exposure.go`

```go
package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/app"
	"github.com/wukfit/equipment-exposure-service/internal/app/query"
	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type RecordExposureInput struct {
	UserID      uuid.UUID
	EquipmentID uuid.UUID
	Duration    int
}

type RecordExposure struct {
	exposures domain.ExposureRepository
	users     domain.UserRepository
	equipment domain.EquipmentRepository
	publisher domain.EventPublisher
	clock     app.Clock
}

func NewRecordExposure(e domain.ExposureRepository, u domain.UserRepository, eq domain.EquipmentRepository, p domain.EventPublisher, c app.Clock) *RecordExposure {
	return &RecordExposure{exposures: e, users: u, equipment: eq, publisher: p, clock: c}
}

func (h *RecordExposure) Handle(ctx context.Context, in RecordExposureInput) (*query.ExposureReadModel, error) {
	user, err := h.users.GetByID(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	equip, err := h.equipment.GetByID(ctx, in.EquipmentID)
	if err != nil {
		return nil, err
	}
	exp, err := domain.NewExposure(user.ID(), equip.ID(), in.Duration, equip.VibrationMagnitude(), h.clock())
	if err != nil {
		return nil, err
	}
	if err := h.exposures.Save(ctx, exp); err != nil {
		return nil, err
	}
	_ = h.publisher.Publish(ctx, domain.ExposureRecorded{
		ExposureID: exp.ID(), UserID: user.ID(), EquipmentID: equip.ID(),
		A8: exp.Partial().A8(), Points: exp.Partial().Points(), RecordedAt: exp.RecordedAt(),
	})
	return &query.ExposureReadModel{Exposure: exp, User: user, Equipment: equip}, nil
}
```

- [ ] **Step 5: slog EventPublisher adapter** — `internal/adapters/events/slog_publisher.go`

```go
package events

import (
	"context"
	"log/slog"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type SlogPublisher struct{ logger *slog.Logger }

func NewSlogPublisher(l *slog.Logger) *SlogPublisher { return &SlogPublisher{logger: l} }

func (p *SlogPublisher) Publish(ctx context.Context, e domain.ExposureRecorded) error {
	p.logger.InfoContext(ctx, "exposure.recorded",
		slog.String("event", "ExposureRecorded"),
		slog.String("exposure_id", e.ExposureID.String()),
		slog.String("user_id", e.UserID.String()),
		slog.String("equipment_id", e.EquipmentID.String()),
		slog.Float64("a8", e.A8),
		slog.Float64("points", e.Points),
		slog.Time("recorded_at", e.RecordedAt),
	)
	return nil
}
```

- [ ] **Step 6: Error-mapping helper** — `internal/adapters/http/errors.go`

```go
package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// writeError maps a sentinel/domain error to an HTTP status + JSON body.
// Explicit default → 500 (no nil-deref, no per-handler duplication).
func writeError(w http.ResponseWriter, logger *slog.Logger, err error) {
	status, slug := http.StatusInternalServerError, "server_error"
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		status, slug = http.StatusNotFound, "user_not_found"
	case errors.Is(err, domain.ErrEquipmentNotFound):
		status, slug = http.StatusNotFound, "equipment_not_found"
	case errors.Is(err, domain.ErrExposureNotFound):
		status, slug = http.StatusNotFound, "exposure_not_found"
	case errors.Is(err, domain.ErrInvalidDuration):
		status, slug = http.StatusUnprocessableEntity, "invalid_duration"
	case errors.Is(err, errBadRequest):
		status, slug = http.StatusBadRequest, "invalid_request"
	}
	if status == http.StatusInternalServerError {
		logger.Error("request failed", slog.String("error", err.Error()))
	}
	writeJSON(w, status, errorBody{Error: slug, Message: err.Error()})
}

var errBadRequest = errors.New("bad request")

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 7: Mappers** — `internal/adapters/http/mappers.go`

```go
package http

import (
	"github.com/wukfit/equipment-exposure-service/internal/adapters/http/api"
	"github.com/wukfit/equipment-exposure-service/internal/app/query"
)

func ptr[T any](v T) *T { return &v }

// toAPIExposure builds the embedded API Exposure from the read model.
// NOTE: field pointer-ness and the a8/points numeric type are whatever oapi
// generated (confirmed in Task 0 Step 3); adjust the assignments to match.
func toAPIExposure(rm *query.ExposureReadModel) api.Exposure {
	e, u, eq := rm.Exposure, rm.User, rm.Equipment
	return api.Exposure{
		Id:       ptr(e.ID()),
		Duration: ptr(e.Duration()),
		A8:       ptr(float32(e.Partial().A8())),
		Points:   ptr(float32(e.Partial().Points())),
		User:     ptr(api.User{Id: ptr(u.ID()), Name: ptr(u.Name())}),
		Equipment: ptr(api.EquipmentItem{
			Id: ptr(eq.ID()), Name: ptr(eq.Name()), VibrationMagnitude: ptr(float32(eq.VibrationMagnitude())),
		}),
	}
}
```

- [ ] **Step 8: Blackbox POST test (outside-in) + test server helper** — `internal/adapters/http/exposure_post_test.go`, `internal/adapters/http/testserver_test.go`

```go
// testserver_test.go — shared harness for blackbox tests
package http_test

import (
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	apphttp "github.com/wukfit/equipment-exposure-service/internal/adapters/http"
	"github.com/wukfit/equipment-exposure-service/internal/app"
	// command/query, memory, events, seed imports
)

func newTestServer(t *testing.T, clock app.Clock, preload ...*someExposureSeed) *httptest.Server {
	// build memory repos seeded with seed.Users()/seed.Equipment(),
	// optionally preload exposures (for summary tests via repo.Save),
	// wire RecordExposure + the three queries + slog publisher into the
	// ServerInterface impl, and return httptest.NewServer(apphttp.NewRouter(deps)).
	// (Concrete wiring mirrors cmd/server/main.go from Task 7 Step 2.)
}
```
```go
// exposure_post_test.go
package http_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wukfit/equipment-exposure-service/internal/app"
	"github.com/wukfit/equipment-exposure-service/internal/seed"
)

func TestPostExposure(t *testing.T) {
	clock := app.Clock(func() time.Time { return time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC) })
	srv := newTestServer(t, clock)
	defer srv.Close()

	body := `{"user_id":"` + seed.BobbyID.String() + `","equipment_id":"` + seed.AirCatID.String() + `","duration":30}`
	resp, err := http.Post(srv.URL+"/exposure", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	// decode JSON; assert a8≈0.525, points==4, embedded user.name=="Bobby Tables",
	// equipment.vibration_magnitude==2.1, and id is a valid uuid.
	// validate body against spec.yaml via kin-openapi (see Task 6 Step for helper).

	// error cases:
	// - unknown user_id   → 404 user_not_found
	// - unknown equipment → 404 equipment_not_found
	// - duration 0        → 422 invalid_duration
	// - malformed JSON / bad uuid → 400 invalid_request
}
```

- [ ] **Step 9: Implement the HTTP handler** — `internal/adapters/http/handlers.go`

```go
package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/adapters/http/api"
	"github.com/wukfit/equipment-exposure-service/internal/app/command"
)

// Server implements api.ServerInterface.
type Server struct {
	deps RouterDeps
}

// RecordExposure handles POST /exposure. (Method name/signature match the
// generated api.ServerInterface; confirm in Task 0 Step 3.)
func (s *Server) RecordExposure(w http.ResponseWriter, r *http.Request) {
	var body api.RecordExposureJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, s.deps.Logger, errBadRequest)
		return
	}
	// body fields are uuid/int per spec; the generated types may already parse
	// uuids. Map into the command input (parse if they are strings).
	in := command.RecordExposureInput{
		UserID:      uuid.UUID(*body.UserId),
		EquipmentID: uuid.UUID(*body.EquipmentId),
		Duration:    *body.Duration,
	}
	rm, err := s.deps.RecordExposure.Handle(r.Context(), in)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAPIExposure(rm))
}
```
Extend `RouterDeps` with `RecordExposure *command.RecordExposure` (and the query handlers, added in later slices), register the generated routes onto the mux in `NewRouter` (use the generated `api.HandlerFromMux`/`api.RegisterHandlers` helper, confirmed in Task 0), keeping `/health` and the middleware chain.

- [ ] **Step 10: Run** — `go test -race -cover ./...` → PASS (command unit test + POST blackbox).

- [ ] **Step 11: Ship** — `feat: record exposure (POST /exposure) with event seam`; open the PR (branch `record-exposure`).

---

## Task 4: GET /exposure/{id} (PR #5)

**Goal:** `GetExposure` query + read-model resolution + handler.

**Files:** Create `internal/app/query/get_exposure.go` (+ test), extend `internal/adapters/http/handlers.go`, add `internal/adapters/http/exposure_get_test.go`.

- [ ] **Step 1: Query test-first** — `get_exposure_test.go`: seed an exposure via repo, assert `Handle` returns a read model with resolved user+equipment; unknown id → `ErrExposureNotFound`.

- [ ] **Step 2: Run red, then implement** — `get_exposure.go`

```go
package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type GetExposure struct {
	exposures domain.ExposureRepository
	users     domain.UserRepository
	equipment domain.EquipmentRepository
}

func NewGetExposure(e domain.ExposureRepository, u domain.UserRepository, eq domain.EquipmentRepository) *GetExposure {
	return &GetExposure{exposures: e, users: u, equipment: eq}
}

func (h *GetExposure) Handle(ctx context.Context, id uuid.UUID) (*ExposureReadModel, error) {
	exp, err := h.exposures.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	user, err := h.users.GetByID(ctx, exp.UserID())
	if err != nil {
		return nil, err
	}
	equip, err := h.equipment.GetByID(ctx, exp.EquipmentID())
	if err != nil {
		return nil, err
	}
	return &ExposureReadModel{Exposure: exp, User: user, Equipment: equip}, nil
}
```

- [ ] **Step 3: Handler** — add `GetExposure(w, r, exposureId)` to `Server`: parse the path UUID (on parse failure → `errBadRequest`), call the query, on success `writeJSON(w, http.StatusOK, toAPIExposure(rm))` (**200**, per design §3). Add `*query.GetExposure` to `RouterDeps`.

- [ ] **Step 4: Blackbox test** — `exposure_get_test.go`: POST then GET the returned id → 200 + matching body (schema-validated); unknown id → 404 `exposure_not_found`; non-uuid path → 400.

- [ ] **Step 5: Run + Ship** — `go test -race ./...` → PASS; `feat: get exposure by id (GET /exposure/{id})`; open the PR (branch `get-exposure`).

---

## Task 5: GET /exposure (PR #6)

**Goal:** `ListExposures` query → `[]ExposureReadModel` → 200 list.

**Files:** Create `internal/app/query/list_exposures.go` (+ test), extend handlers, add `internal/adapters/http/exposure_list_test.go`.

- [ ] **Step 1: Query test-first** — seed two exposures (different users), assert both returned with resolved associations, ordered by recordedAt.

- [ ] **Step 2: Run red, then implement** — `list_exposures.go`

```go
package query

import (
	"context"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type ListExposures struct {
	exposures domain.ExposureRepository
	users     domain.UserRepository
	equipment domain.EquipmentRepository
}

func NewListExposures(e domain.ExposureRepository, u domain.UserRepository, eq domain.EquipmentRepository) *ListExposures {
	return &ListExposures{exposures: e, users: u, equipment: eq}
}

func (h *ListExposures) Handle(ctx context.Context) ([]*ExposureReadModel, error) {
	exps, err := h.exposures.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*ExposureReadModel, 0, len(exps))
	for _, exp := range exps {
		user, err := h.users.GetByID(ctx, exp.UserID())
		if err != nil {
			return nil, err
		}
		equip, err := h.equipment.GetByID(ctx, exp.EquipmentID())
		if err != nil {
			return nil, err
		}
		out = append(out, &ExposureReadModel{Exposure: exp, User: user, Equipment: equip})
	}
	return out, nil
}
```

- [ ] **Step 3: Handler** — `GetExposures(w, r)`: call query, map each via `toAPIExposure`, `writeJSON(w, 200, []api.Exposure{...})`. Add `*query.ListExposures` to `RouterDeps`.

- [ ] **Step 4: Blackbox test** — empty → `200 []`; after two POSTs → 200 with 2 items (schema-validated).

- [ ] **Step 5: Run + Ship** — `feat: list exposures (GET /exposure)`; open the PR (branch `list-exposures`).

---

## Task 6: GET /users/{userId}/exposure-summary (PR #7)

**Goal:** Window resolution + `GetUserExposureSummary` (aggregate via RSS) + handler. The domain-rich slice.

**Files:** Create `internal/app/window.go` (+ test), `internal/app/query/get_user_exposure_summary.go` (+ test), extend handlers, add `internal/adapters/http/summary_test.go`, and a kin-openapi validation helper for the blackbox suite.

- [ ] **Step 1: Window resolution test-first** — `window_test.go`

```go
package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveWindow(t *testing.T) {
	now := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	clock := Clock(func() time.Time { return now })

	t.Run("both omitted → trailing 24h", func(t *testing.T) {
		s, e, err := ResolveWindow(clock, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, now.Add(-24*time.Hour), s)
		assert.Equal(t, now, e)
	})
	t.Run("start only → end=now", func(t *testing.T) {
		start := now.Add(-72 * time.Hour)
		s, e, err := ResolveWindow(clock, &start, nil)
		require.NoError(t, err)
		assert.Equal(t, start, s)
		assert.Equal(t, now, e)
	})
	t.Run("end only → start=end-24h", func(t *testing.T) {
		end := now.Add(-48 * time.Hour)
		s, e, err := ResolveWindow(clock, nil, &end)
		require.NoError(t, err)
		assert.Equal(t, end.Add(-24*time.Hour), s)
		assert.Equal(t, end, e)
	})
	t.Run("start after end → error", func(t *testing.T) {
		start, end := now, now.Add(-time.Hour)
		_, _, err := ResolveWindow(clock, &start, &end)
		assert.Error(t, err)
	})
}
```

- [ ] **Step 2: Run red, then implement** — `internal/app/window.go`

```go
package app

import (
	"errors"
	"time"
)

const defaultWindow = 24 * time.Hour

var ErrInvalidWindow = errors.New("starting_at must be before ending_at")

// ResolveWindow applies the default-window rules (design §3).
func ResolveWindow(clock Clock, start, end *time.Time) (time.Time, time.Time, error) {
	now := clock()
	var s, e time.Time
	switch {
	case start == nil && end == nil:
		s, e = now.Add(-defaultWindow), now
	case start != nil && end == nil:
		s, e = *start, now
	case start == nil && end != nil:
		s, e = end.Add(-defaultWindow), *end
	default:
		s, e = *start, *end
	}
	if s.After(e) {
		return time.Time{}, time.Time{}, ErrInvalidWindow
	}
	return s, e, nil
}
```

- [ ] **Step 3: Summary query test-first** — `get_user_exposure_summary_test.go`: seed (via repo) two in-window exposures for Bobby (AirCat 30m + JCB 120m) and one out-of-window; assert a8≈2.0678, points==68, user resolved; unknown user → `ErrUserNotFound`; user with no exposures → zeros.

- [ ] **Step 4: Run red, then implement** — `get_user_exposure_summary.go`

```go
package query

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

type UserExposureSummary struct {
	User    *domain.User
	Partial domain.PartialExposure
}

type GetUserExposureSummary struct {
	exposures domain.ExposureRepository
	users     domain.UserRepository
}

func NewGetUserExposureSummary(e domain.ExposureRepository, u domain.UserRepository) *GetUserExposureSummary {
	return &GetUserExposureSummary{exposures: e, users: u}
}

func (h *GetUserExposureSummary) Handle(ctx context.Context, userID uuid.UUID, start, end time.Time) (*UserExposureSummary, error) {
	user, err := h.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	exps, err := h.exposures.ListByUserInWindow(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}
	parts := make([]domain.PartialExposure, 0, len(exps))
	for _, e := range exps {
		parts = append(parts, e.Partial())
	}
	return &UserExposureSummary{User: user, Partial: domain.Aggregate(parts)}, nil
}
```

- [ ] **Step 5: Handler** — `GetUserExposureSummary(w, r, userId, params)`: parse the userId UUID (bad → `errBadRequest`); the generated `params` carry optional `StartingAt`/`EndingAt` — if they are strings, parse RFC3339 (bad → `errBadRequest`); call `app.ResolveWindow`, then the query; map to `api.ExposureSummary{A8, Points, User}`; **200**. Map `app.ErrInvalidWindow` → 400. Add a summary mapper + `*query.GetUserExposureSummary` and `app.Clock` to `RouterDeps`.

- [ ] **Step 6: kin-openapi validation helper + blackbox test** — `internal/adapters/http/openapi_test.go` loads `spec.yaml`, and a helper validates a captured response against the operation's response schema. `summary_test.go` exercises: in-window aggregation (a8≈2.0678, points 68), window boundary inclusivity, user isolation (Alice's exposures excluded from Bobby's summary), empty window → zeros, default window, unknown user → 404. Retrofit the validation helper into the earlier blackbox tests (POST/GET/list).

- [ ] **Step 7: Run + Ship** — `go test -race -cover ./...` → PASS; `feat: user exposure summary (GET /users/{id}/exposure-summary)`; open the PR (branch `exposure-summary`).

---

## Task 7: Finalisation (PR #8)

**Goal:** Real `cmd/server/main.go` wiring of all handlers, full README, coverage report, compose smoke test.

**Files:** Modify `cmd/server/main.go`; rewrite `README.md`; optionally add `docs/` notes.

- [ ] **Step 1:** Wire everything in `main.go` — construct memory repos preloaded with `seed.Equipment()`/`seed.Users()`, the slog publisher, `app.SystemClock`, the command + three query handlers, populate `RouterDeps`, register routes. Confirm `docker compose up` serves all four endpoints.

- [ ] **Step 2:** Rewrite `README.md` — overview; **run** (`docker compose up` / `make run`); **test** (`make test`, coverage); the **seeded UUIDs** + copy-paste `curl` examples for all four endpoints; the **documented decisions** (corrected formula w/ before/after, 200-not-201, RSS aggregation, default 24h window, internal `recordedAt`, added error schema); the **EDA** section (publish `ExposureRecorded` + derived EAV/ELV alerts; consume Equipment/User lifecycle; why no user/equipment CRUD); **compromises & next steps** (in-memory → Postgres drop-in).

- [ ] **Step 3:** `go test -race -cover ./...`; capture coverage (`go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out`). Note domain coverage in the README.

- [ ] **Step 4:** Smoke test the four endpoints against `docker compose up` with the documented curls.

- [ ] **Step 5: Ship** — `docs: finalise README + wire server`; open the PR (branch `finalisation`).

---

## Task 8: Postgres adapter (STRETCH — PR #9, only if time)

**Goal:** A `postgres` `ExposureRepository`/`EquipmentRepository`/`UserRepository` passing the **same** `contracttest.RunExposureRepository` suite, proving port substitutability.

**Files:** Create `internal/adapters/repository/postgres/*` (pgx + sqlc), `db/migrations/*` (goose), extend `docker-compose.yml` (app + postgres + healthcheck), a `--db=postgres` config switch in `main.go`, and a testcontainers-backed `postgres_test.go` invoking the contract suite.

- [ ] **Step 1:** Add deps (pgx/v5, goose, sqlc as a tool, testcontainers). Migration: `exposures` table (id, user_id, equipment_id, duration, a8, points, recorded_at).
- [ ] **Step 2:** sqlc queries (insert, get by id, list, list-by-user-in-window) → generated code.
- [ ] **Step 3:** Adapter implementing the ports, mapping rows → `domain.NewExposureFromStore`.
- [ ] **Step 4:** `postgres_test.go` (testcontainers) runs `contracttest.RunExposureRepository` against the real DB.
- [ ] **Step 5:** Config switch + compose app+db (depends_on healthcheck). Verify `docker compose up` boots both and the app works against Postgres.
- [ ] **Step 6: Ship** — `feat: add postgres repository adapter`; open the PR (branch `postgres-adapter`).

---

## Self-review notes

- **Spec coverage:** POST /exposure → Task 3; GET /exposure → Task 5; GET /exposure/{id} → Task 4; GET /users/{id}/exposure-summary → Task 6. All four schemas (`Exposure`, `ExposureSummary`, `User`, `EquipmentItem`) mapped in Tasks 3/6. Resolved decisions from design §3 are each implemented (calc Task 1; 200 Task 4; window Task 6; RFC3339 parse Task 6; RSS Task 1/6; `recordedAt` Task 2). EDA seam Task 3; README EDA Task 7.
- **Generated-name caveat:** Task 0 Step 3 is the single point where exact oapi-codegen names/types (request-body type, `ServerInterface` method names, pointer-ness, float32-vs-float64 for `number`, route-registration helper) are pinned; Tasks 3-6 adjust mapper/handler assignments to match. This is the one unavoidable "confirm at generation" — generated identifiers are not knowable a priori.
- **Type consistency:** `ExposureReadModel` (query pkg) used by command + queries; `RecordExposureInput`, `RouterDeps` (grown additively), `writeError`/`writeJSON`/`errBadRequest`, `app.Clock`/`ResolveWindow` consistent across tasks.
