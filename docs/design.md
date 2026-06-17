# HAVS Equipment Exposure Service — Design

- **Status:** Approved (design)
- **Contract:** `spec.yaml`

## 1. Overview

A runnable HTTP API in Go for tracking worker exposure to Hand & Arm Vibration
Syndrome (HAVS). Clients record *exposures* (a user using a piece of equipment
for a duration) and retrieve a user's aggregated exposure over a time window.
Two metrics matter per the HSE model: **Partial Exposure Points** and **Partial
Exposure A(8)**.

The build prioritises Domain Driven Design, clean (ports & adapters)
architecture, comprehensive test coverage, and single-command runnability.

## 2. Guiding constraints

- **Satisfy `spec.yaml` as written.** Deviations require a strong, documented
  reason. Underspecified behaviour is resolved as a *documented assumption*, not
  a deviation. No endpoints beyond the spec (notably: no `User`/`EquipmentItem`
  CRUD, no auth — the spec defines neither).
- **Comprehensive tests.** We aim for thorough coverage across all layers (see
  §10), not a minimal smoke suite.
- Stay close to stdlib where reasonable; frameworks allowed where they earn it.

## 3. Resolved spec decisions (all documented in the README)

| Item | Decision | Rationale |
|---|---|---|
| HAVS calculation | **Corrected float maths** (see §5.3) | The provided functions use Go integer division `(triggerTime/60)/8`, truncating to **0 for any exposure under 8 hours** — the API would return zeros for virtually all real input. Deliberate, documented correction of an obvious translation bug. |
| `GET /exposure/{id}` status | **200** (spec says 201) | 201 = "Created"; treated as a copy-paste typo from the POST. |
| Summary window default | both omitted → trailing **24h** (`now-24h .. now`); `start` only → `end=now`; `end` only → `start=end-24h` | Brief says "usually 24h windows"; params are optional. |
| `starting_at`/`ending_at` parsing | accept **RFC3339 datetime** | Examples are full timestamps despite `format: date`; matches real usage. |
| A(8) aggregation (summary) | **root-sum-of-squares** `√Σa8ᵢ²`; **points sum linearly** | HSE-correct energy combination; plain-summing A(8) is physically wrong. Spec is silent → documented assumption. |
| Exposure timestamp | internal **`recordedAt`** = server `now()` at record time; **not serialised** | The `Exposure` response schema has no timestamp, but the summary filters by a window, so one is required internally. POST body carries no timestamp either. |

## 4. Architecture (ports & adapters)

```
cmd/server                          wiring, config, graceful shutdown
internal/domain                     Exposure aggregate, User, EquipmentItem,
                                    PartialExposure VO, repository PORTS,
                                    EventPublisher port, ExposureRecorded event,
                                    sentinel errors, the HAVS calc + aggregation
internal/app/command                RecordExposure
internal/app/query                  GetExposure, ListExposures,
                                    GetUserExposureSummary  (+ ExposureReadModel)
internal/adapters/http              oapi-codegen server iface, request→cmd/query
                                    mapping, ExposureReadModel→response mapping,
                                    error mapping, middleware
internal/adapters/repository/memory in-memory adapters (wired now)
internal/adapters/repository/postgres  STRETCH only, if time allows
internal/adapters/events            slog EventPublisher adapter
internal/seed                       fixed seed fixtures (shared app + tests)
```

Dependency rule: adapters depend on app/domain; app depends on domain; domain
depends on nothing. The app layer must **not** import the generated HTTP
`types` package.

## 5. Domain model

### 5.1 Entities / aggregates (private fields, factory constructors, getters)

- **`EquipmentItem`** — `id`, `name`, `vibrationMagnitude` (m/s²). Seeded,
  read-only catalog. Constructor validates: name non-empty, magnitude > 0.
- **`User`** — `id`, `name`. Seeded. Constructor validates name non-empty.
- **`Exposure`** (aggregate root) — `id`, `userID`, `equipmentID`, `duration`
  (minutes), `partial` (`PartialExposure`), `recordedAt`. References other
  aggregates **by ID**, never by embedding. Constructor validates
  `duration > 0` and computes its `PartialExposure` once at construction (its
  invariant), from the equipment's magnitude + duration.

### 5.2 Value object — `PartialExposure`

Holds `a8` and `points`; home of the corrected maths and the aggregation logic.
Reused per-exposure and in the summary, which earns it VO status. (`duration`
and `magnitude` stay as validated primitives — no single-use VO wrappers.)

### 5.3 The HAVS calculation (corrected float)

```go
func NewPartialExposure(magnitude float64, minutes int) PartialExposure {
    hours  := float64(minutes) / 60.0
    a8     := magnitude * math.Sqrt(hours/8.0)
    points := math.Round(math.Pow(magnitude/2.5, 2) * (hours/8.0) * 100)
    return PartialExposure{a8: a8, points: points}
}
```

Reference values (pinned in tests): AirCat 2.1 @ 30min → a8 `0.525`, points `4`;
JCB 4.0 @ 2h → a8 `2.0`, points `64`.

### 5.4 Aggregation

```go
// combined A(8) = √Σa8ᵢ²  (RSS) ;  points sum linearly
func Aggregate(parts []PartialExposure) PartialExposure
```
Empty → zeros; single → identity; order-independent. A(8) values are combined
from the **unrounded** per-exposure `a8`; points are the **sum of the
per-exposure (already-rounded) `points`** — consistent with HSE per-activity
points rounding. The combined A(8) is not re-rounded.

### 5.5 Repository ports (domain)

```go
ExposureRepository:  Save · GetByID · List · ListByUserInWindow(userID, start, end)
EquipmentRepository: GetByID
UserRepository:      GetByID
```

### 5.6 Sentinel errors

`ErrUserNotFound`, `ErrEquipmentNotFound`, `ErrExposureNotFound`,
`ErrInvalidDuration`, `ErrInvalidMagnitude`.

## 6. Application layer & data flow

All domain construction and association resolution happens here, never in the
HTTP layer. The HTTP layer only parses, maps to a command/query, maps the result
to a response, and maps errors to status codes.

- **`POST /exposure` → `RecordExposureCommand{UserID, EquipmentID, Duration}`**
  1. Load `User` (→ `ErrUserNotFound`), load `EquipmentItem`
     (→ `ErrEquipmentNotFound`).
  2. `NewExposure(userID, equipmentID, duration, equipment.Magnitude())` —
     validates, computes `PartialExposure`, sets `recordedAt = now()`.
  3. `ExposureRepository.Save`.
  4. Publish `ExposureRecorded` (see §8).
  5. Return `ExposureReadModel{exposure, user, equipment}` → **201**.
- **`GET /exposure` → `ListExposures`** — list, resolve user+equipment per
  exposure into `[]ExposureReadModel` → **200**. (N catalog lookups in-memory;
  a JOIN if Postgres lands.)
- **`GET /exposure/{id}` → `GetExposure`** — `GetByID`
  (→ `ErrExposureNotFound`), resolve → **200**.
- **`GET /users/{userId}/exposure-summary` → `GetUserExposureSummary{UserID, Start, End}`**
  1. Load `User` (→ `ErrUserNotFound` → 404) — needed because the summary
     embeds `user`.
  2. `ListByUserInWindow(userID, start, end)`, then `Aggregate(partials)`.
  3. Empty window → `a8:0, points:0` (not 404) → **200**.

### 6.1 Read model

`ExposureReadModel{ exposure, user, equipment }` is a CQRS **read-side
projection**, distinct from the `Exposure` write aggregate. It exists because
(a) the aggregate references by ID so the read side must compose the embedded
representation, (b) the app layer must not depend on the generated HTTP types,
and (c) `ListExposures` needs a slice of a composed type. The HTTP mapper turns
it into `types.Exposure`.

## 7. HTTP layer & error handling

- **Framework: stdlib `net/http`** via oapi-codegen `std-http-server`; Go 1.22
  `ServeMux` for routing. Middleware chain: request-ID → structured (slog)
  logging → panic recovery.
- **Error response** (spec defines none — documented gap-fill):
  `{ "error": "<slug>", "message": "<human readable>" }`.
- **Single mapping helper** (`errors.Is`-based, explicit `default → 500` — avoids
  a nil-deref error-handling pitfall and per-handler duplication):

| Error | Status |
|---|---|
| bad JSON / bad UUID / unparseable date param | 400 |
| `ErrInvalidDuration` (well-formed but invalid) | 422 |
| `ErrUserNotFound` / `ErrEquipmentNotFound` / `ErrExposureNotFound` | 404 |
| unmatched | 500 (logged) |

Validation lives in domain constructors; no business logic in the HTTP layer.

## 8. Events / EDA

**In-code seam (included):**
- `domain.ExposureRecorded` event: `exposureId, userId, equipmentId, a8, points, recordedAt`.
- `domain.EventPublisher` port; `RecordExposure` publishes after a successful
  `Save`.
- `adapters/events` slog adapter logs the event as structured JSON; a fake
  publisher in tests asserts emission.

**README discussion:**
- **Publish:** `ExposureRecorded`; derived downstream `ExposureActionValueReached`
  / `ExposureLimitExceeded` against the HSE EAV (100 pts / A(8) 2.5) and ELV
  (400 pts / A(8) 5.0) — the natural alerting use case.
- **Consume:** `EquipmentRegistered`/`EquipmentUpdated`,
  `UserCreated`/`UserDeactivated` from other bounded contexts — which explains
  why this service owns no user/equipment CRUD: that data is owned elsewhere and
  would arrive via events.

## 9. Seeding & config

- **Seed set** (fixed, documented UUIDs; shared by app startup and tests),
  reusing the spec's own example IDs so they are real working IDs:
  - `2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49` — AirCat - Drill - 4337 (2.1 m/s²)
  - `36603447-2f30-41b1-a908-526c0b6f1755` — JCB - Hydraulic Breaker - CEJCBHM25 (4.0 m/s²)
  - `713be58e-0d79-4df2-a85c-9f44ca513a7d` — user "Bobby Tables"
  - one additional sample user (fixed UUID assigned in the seed package) for
    user-isolation tests
- **Config:** env-driven (`PORT`, `LOG_LEVEL`) with sane defaults. No DB config
  unless the Postgres stretch lands. No secrets, no auth.
- **Versioning:** Go version pinned consistently across `go.mod` / `toolchain` /
  Dockerfile.

## 10. Testing strategy (comprehensive; outside-in, test-first)

**Tier 1 — Domain unit tests (testify, table-driven; ~full coverage):**
calc across durations/equipment with exact pinned values incl. boundaries (8h,
sub-hour, points rounding, large durations); `Aggregate` (empty/single/multiple/
order-independence); all constructor validation branches; window-resolution
branches incl. `start > end` error.

**Tier 2 — Blackbox API contract tests (`httptest`; exhaustive per endpoint):**
every status branch and malformed-input kind; summary semantics in depth (window
boundary inclusivity, user isolation, empty→zeros, default window); list ordering
and empty list. **OpenAPI response validation against `spec.yaml` via
`kin-openapi`** (the provable-fidelity artifact).

**Tier 3 — Adapter/integration tests:** in-memory repo
(`Save`/`GetByID`/`List`/`ListByUserInWindow`, window filtering, not-found,
**concurrency under `-race`**); mappers (`ExposureReadModel` → `types.Exposure`);
error-mapping helper (each sentinel → expected status).

**Tier 4 — Postgres (only if the stretch lands):** testcontainers, running the
same suite as Tier 3.

**Substitutability:** a shared `ExposureRepository` contract test — one table of
behaviours run against every adapter (memory now; Postgres if added).

**Measurement:** `go test -race -cover ./...`; report coverage. Excluded:
generated code (oapi-codegen), trivial getters, `main.go` wiring.

## 11. Runnability

- `docker compose up` boots the service (in-memory → app-only, no DB container).
  Multi-stage Dockerfile, non-root, pinned base.
- `Makefile`: `run`, `test` (`go test -race -cover ./...`), `generate`
  (oapi-codegen), `docker`.
- **README** (deliverable): overview; documented decisions (§3); seeded UUIDs +
  example `curl`s; run/test instructions; EDA section (§8); compromises & next
  steps.

## 12. Out of scope / non-goals

- No `User`/`EquipmentItem` CRUD endpoints (not in spec).
- No authentication/authorisation (not in spec).
- No real message broker (the EDA seam logs events; broker wiring is described,
  not built).

## 13. Delivery in slices (one PR each)

Built incrementally — each slice is an isolated, independently verifiable PR
against `main`, ending green and committed; never one-shot.

0. **Walking skeleton** — module, layout, oapi-codegen generate, `net/http`
   server + middleware, `/health`, Dockerfile + compose + Makefile.
1. **HAVS maths in isolation** — `PartialExposure` + `Aggregate`, exhaustively
   unit-tested (highest correctness risk, de-risked first).
2. **Entities, ports, seed, in-memory repo** — incl. shared repo contract test
   and concurrency.
3. **`POST /exposure`** — first full vertical (command + HTTP + mapping + error
   helper + `EventPublisher` seam), outside-in.
4. **`GET /exposure/{id}`**.
5. **`GET /exposure`**.
6. **`GET /users/{id}/exposure-summary`** — window parsing/defaults + aggregation.
7. **Finalisation** — README, coverage report, compose smoke test.
8. **Postgres adapter** (stretch) — same contract suite via testcontainers.

## 14. Compromises & next steps (for the README)

- **Persistence is in-memory.** The `ExposureRepository` port makes Postgres a
  drop-in adapter; building it (pgx + sqlc + goose, docker compose app+db,
  testcontainers reusing the contract suite) is the first next step and a
  time-boxed stretch in this build.
- The corrected formula and the RSS aggregation are documented assumptions to
  confirm with the domain owner.
