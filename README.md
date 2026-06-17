# Equipment Exposure Service

An HTTP API in Go for tracking worker exposure to **Hand & Arm Vibration
Syndrome (HAVS)**. Clients record *exposures* (a user operating a piece of
equipment for some duration) and read back a user's aggregated exposure over a
time window. Two numbers matter, both from the HSE vibration model: **Partial
Exposure Points** and **Partial Exposure A(8)**.

The API contract lives in [`spec.yaml`](./spec.yaml) (OpenAPI 3.0.3). The full
design write-up (architecture, domain model, the spec decisions I made, and the
testing strategy) is in [`docs/design.md`](./docs/design.md).

It's built with Domain-Driven Design and a clean ports-and-adapters layout, it's
tested at every layer, and it runs with a single command.

## Quick start

### Run

```bash
docker compose up --build        # builds and serves on :8080 (in-memory store)
# ...or without Docker:
make run                         # go run ./cmd/server
```

Configuration comes from the environment: `PORT` (default `8080`) and `LOG_LEVEL`
(`debug|info|warn|error`, default `info`). There's no database, no secrets, and
no auth.

```bash
curl -s localhost:8080/health    # {"status":"ok"}
```

### Test

```bash
make test                        # go test -race -cover ./...
make generate                    # regenerate the API package from spec.yaml
```

## Seeded data

The in-memory catalog is seeded at startup with a fixed set of fixtures. The IDs
reuse the spec's own examples, so they're real, working IDs you can paste
straight into requests:

| Kind | ID | Detail |
|---|---|---|
| Equipment | `2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49` | AirCat - Drill - 4337, **2.1 m/s²** |
| Equipment | `36603447-2f30-41b1-a908-526c0b6f1755` | JCB - Hydraulic Breaker - CEJCBHM25, **4.0 m/s²** |
| User | `713be58e-0d79-4df2-a85c-9f44ca513a7d` | Bobby Tables |
| User | `a52a3c1e-7b2d-4c9a-9f0e-1d6b8c4f2a10` | Alice Stone |

> The seed stands in for users and equipment that, in production, would be
> **owned by other bounded contexts** and arrive via `UserCreated` /
> `EquipmentRegistered` events (see **Events** below). This service treats them as
> **read-only reference data** and deliberately owns no CRUD for them. That
> matches the brief, which describes the entities but doesn't ask for user or
> equipment management.

## API

All four endpoints are defined in `spec.yaml`. Exposure responses embed the
associated `user` and `equipment`. The summary response embeds `user` only. Every
error shares one shape: `{ "error": "<slug>", "message": "…" }`.

### `POST /exposure`: record an exposure → **201**

```bash
curl -s -X POST localhost:8080/exposure \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"713be58e-0d79-4df2-a85c-9f44ca513a7d","equipment_id":"2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49","duration":30}'
```
```json
{"id":"…","duration":30,"a8":0.525,"points":4,
 "user":{"id":"713be58e-…","name":"Bobby Tables"},
 "equipment":{"id":"2e85d43d-…","name":"AirCat - Drill - 4337","vibration_magnitude":2.1}}
```

### `GET /exposure/{id}`: fetch one → **200**

```bash
curl -s localhost:8080/exposure/<id-from-the-POST>
```

### `GET /exposure`: list all → **200** (ordered by recorded time, then id)

```bash
curl -s localhost:8080/exposure
```

### `GET /users/{userId}/exposure-summary`: aggregate over a window → **200**

```bash
# default window = trailing 24h [now-24h, now)
curl -s localhost:8080/users/713be58e-0d79-4df2-a85c-9f44ca513a7d/exposure-summary

# explicit half-open window [starting_at, ending_at), RFC3339, normalised to UTC
curl -s "localhost:8080/users/713be58e-0d79-4df2-a85c-9f44ca513a7d/exposure-summary?starting_at=2025-01-01T00:00:00Z&ending_at=2025-12-31T23:59:59Z"
```

Two in-window exposures for one user (AirCat 30 min plus JCB 2 h) summarise to:
```json
{"a8":2.0677586,"points":68,"user":{"id":"713be58e-…","name":"Bobby Tables"}}
```

> Exposures are stamped with the server's current time, so the **window has to
> contain "now"** to include freshly-recorded exposures. The default trailing-24h
> window does. A window that sits entirely in the past (for example the explicit
> 2025 window above, on a server running later) returns `{"a8":0,"points":0,…}`.

**Try it end to end.** Record an AirCat (30 min) and a JCB (2 h) for Bobby, then
summarise with the default window (both land inside the trailing 24h):

```bash
BOB=713be58e-0d79-4df2-a85c-9f44ca513a7d
curl -s -XPOST localhost:8080/exposure -H 'Content-Type: application/json' \
  -d "{\"user_id\":\"$BOB\",\"equipment_id\":\"2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49\",\"duration\":30}" >/dev/null
curl -s -XPOST localhost:8080/exposure -H 'Content-Type: application/json' \
  -d "{\"user_id\":\"$BOB\",\"equipment_id\":\"36603447-2f30-41b1-a908-526c0b6f1755\",\"duration\":120}" >/dev/null
curl -s localhost:8080/users/$BOB/exposure-summary    # a8 ≈ 2.0678, points 68
```

### Error contract

| Status | When | Example slug |
|---|---|---|
| `400` | malformed JSON, a missing/wrong-typed/trailing body field, a bad UUID or date param, `starting_at` after `ending_at` | `invalid_request`, `invalid_window` |
| `404` | unknown user, equipment, or exposure | `user_not_found`, `equipment_not_found`, `exposure_not_found` |
| `422` | well-formed but an invalid value (e.g. `duration: 0`) | `invalid_duration` |
| `500` | unexpected error, or a referential data-consistency fault (generic body, detail logged) | `server_error` |

## The HAVS calculation

Per exposure, from a vibration magnitude `m` (m/s²) and a duration `t` (minutes,
with `h = t/60`):

```
a8     = m · √(h / 8)
points = round( (m / 2.5)² · (h / 8) · 100 )
```

Aggregating a user's exposures over a window:

```
combined A(8) = √Σ a8ᵢ²     (root-sum-of-squares; vibration energies add)
combined pts  = Σ pointsᵢ    (HSE points are additive)
```

The combined A(8) is built from the **unrounded** per-exposure `a8` values.
Points are the sum of the already-rounded per-activity points. An empty window
gives zeros.

> **Why RSS for A(8)?** A(8) is an RMS acceleration, which is an energy-like
> quantity, so doses combine in quadrature rather than by plain addition.
> Plain-summing A(8) would overstate the dose, and it's physically wrong.

## Documented decisions

`spec.yaml` is the single source of truth. The corrections, additions, and
constraints below are applied directly to it, and the implementation, codegen,
and response validation all agree with it.

| Decision | Rationale |
|---|---|
| **Corrected float maths** | The brief's reference code used Go *integer* division `(t/60)/8`, which truncates to **0 for any exposure under 8 hours**, so the API would return zeros for almost all real input. I corrected it to the float form the maths intends. |
| **RSS for A(8), linear sum for points** | The HSE-correct way to combine the energy (see above). The spec was silent on it, so this is a documented assumption. |
| **`GET /exposure/{id}` → 200** (spec corrected from `201`) | 201 *Created* is wrong for fetching a resource that already exists. |
| **Window params `date-time`** (spec corrected from `date`) | The spec's own examples are RFC3339 instants, and a calendar date can't express a window boundary. Normalised to UTC. |
| **Half-open window `[start, end)`** | An instant sitting exactly on a boundary isn't counted twice across adjacent windows. |
| **Default window** | both omitted gives a trailing 24h `[now-24h, now)`; `start` only gives `end=now`; `end` only gives `start=end-24h`; `start > end` is a 400. |
| **Internal `recordedAt` = server `now()`** | The response and POST schemas carry no timestamp, but the summary filters by a window, so the service needs one internally. It comes from an injected `Clock` and isn't serialised. |
| **`ErrorResponse` schema and `required` lists added** | The supplied spec declared only success responses, with no required fields. Adding the error contract and the required fields means the whole surface, success and error, is validated against the spec in tests. |
| **POST body has no `required` list** (deliberate) | Required-ness is enforced at the boundary, so an absent field returns a clean **400**, kept separate from the **422** you get for a present-but-invalid value. |
| **OpenAPI 3.0.3** (not 3.1) | `oapi-codegen` doesn't support 3.1 yet. The spec uses no 3.1-only features, so 3.0.3 keeps codegen and validation warning-free. |
| **Referential miss on a read → 500** | A stored exposure whose user or equipment can't be resolved is a server-side data-consistency fault, not a client 404. It can't happen today with the immutable seed catalog, but the handling is correct once the catalog becomes mutable. |

## Architecture

Ports and adapters, DDD. The dependency rule holds firm: `adapters → app →
domain`, and `domain` depends on nothing. The app layer never imports the
generated HTTP types. Ports live in the **application layer** (consumer-side,
next to the use cases that own them) rather than in the domain, because
persistence and event publication are application concerns and I wanted the
domain to stay pure.

```
cmd/server                              wiring, config, graceful shutdown
internal/domain                         Exposure aggregate, User, EquipmentItem,
                                        PartialExposure VO (HAVS maths + Aggregate),
                                        ExposureRecorded event, sentinels
internal/app                            application ports (ExposureStore, UserDirectory,
                                        EquipmentCatalog, EventPublisher), Clock, window
internal/app/command · query            RecordExposure; GetExposure, ListExposures,
                                        GetUserExposureSummary (+ ExposureReadModel)
internal/adapters/httpapi               oapi-codegen server, request→cmd/query and
                                        read-model→response mapping, error mapping,
                                        middleware
internal/adapters/repository/memory     in-memory adapters (behind the ports)
internal/adapters/events                slog EventPublisher adapter
internal/seed                           fixed seed fixtures (shared by app + tests)
internal/contracttest                   shared repository contract suite
```

## Events (event-driven architecture)

There's an in-code seam, included and exercised. A real broker is described, not
built.

- **Publish:** `RecordExposure` emits `ExposureRecorded` after a successful save,
  through the `EventPublisher` port. A `slog` adapter logs it as structured JSON,
  and a fake publisher asserts emission in tests. Natural downstream consumers
  would derive `ExposureActionValueReached` and `ExposureLimitExceeded` against
  the HSE **EAV** (100 pts / A(8) 2.5) and **ELV** (400 pts / A(8) 5.0). That's
  the alerting use case.
- **Consume:** `EquipmentRegistered`/`EquipmentUpdated` and
  `UserCreated`/`UserDeactivated` from other bounded contexts. This is why the
  service owns no user or equipment CRUD: that data is owned elsewhere and arrives
  via events.

## Observability

Structured logging is built in. Metrics and distributed tracing are deliberate
next steps: they're out of scope for the brief, and a full stack (Prometheus, a
collector, Jaeger) would work against single-command runnability. The seams for
both are already there.

**Built in (logging):**
- **Structured JSON logs** through `log/slog`, with the level set by `LOG_LEVEL`
  (`debug|info|warn|error`).
- **Per-request correlation.** A `requestID` middleware mints a UUID per request,
  returns it as `X-Request-ID`, and threads it through the request `context`.
- **Access and latency logging:** method, path, status, `elapsed_ms`,
  `request_id`.
- **Panic recovery.** It recovers, logs, and returns a clean `500` instead of
  dropping the connection. Errors that map to `5xx` are logged in full while the
  client gets only a generic body, so nothing internal leaks.

**Deferred, with the seam identified:**
- **Metrics.** A Prometheus `/metrics` endpoint (a request counter and a latency
  histogram) drops into the existing middleware chain. The `statusWriter` that
  already captures the response status is the hook.
- **Distributed tracing.** `context.Context` is threaded end to end (handler →
  app → repository), so OpenTelemetry is wiring rather than refactoring: an
  `otelhttp` handler at the edge, a tracer provider with an OTLP exporter, and
  optionally an slog handler that stamps the active trace and span ID onto every
  log line, joining logs to traces through the correlation seam above.

## Testing

Tests are outside-in and test-first, across four tiers (see design §10):

- domain unit tests with pinned HSE reference values,
- blackbox API contract tests (`httptest`) covering every status branch,
- a shared repository **contract suite** (run under `-race`, including the
  50-goroutine concurrency check and the half-open boundary) that any adapter has
  to satisfy,
- mapper and error-mapping unit tests.

Every blackbox response is validated against `spec.yaml` with `kin-openapi`, body
and status, for both success and error responses.

`go test -race -cover ./...`. Coverage of the hand-written packages:

| Package | Coverage |
|---|---|
| `internal/domain` | 84.4% |
| `internal/app` | 90.9% |
| `internal/app/command` | 92.9% |
| `internal/app/query` | 94.1% |
| `internal/adapters/httpapi` | 94.2% |
| `internal/adapters/repository/memory` | 91.8% |
| `internal/adapters/events` · `internal/seed` | 100% |

Generated code (`api.gen.go`) and the `main.go` wiring are left out of the
meaningful coverage, so the aggregate `total` understates the tested surface.

## Compromises & next steps

Everything below is a deliberate trade-off for a time-boxed take-home (the brief
budgets roughly 4 hours).

- **No auth, no secret management, no database.** Authentication and
  authorisation, secret handling, and a real datastore are all things a
  production service needs, but adding them here would over-engineer the exercise
  and pull focus from the part that's actually being assessed: the domain model
  and the architecture. The in-memory store keeps the thing runnable with a
  single command and no external dependencies.
- **Persistence is in-memory.** The `app.ExposureStore` port makes Postgres a
  drop-in adapter. The first next step is `pgx` + `sqlc` + `goose`, a compose
  `app + db`, and the **same** `contracttest` suite run through testcontainers,
  which proves substitutability with no new test code.
- **`a8` and `points` serialise as `float32`** (the spec's `type: number`). All
  computation is `float64`; only the response boundary narrows, and JSON still
  renders clean shortest-round-trip values. Moving to `float64` on the wire is a
  one-line `format: double` spec change if exact wire precision is ever wanted.
- The **corrected formula** and the **RSS aggregation** are documented
  assumptions I'd confirm with the domain owner.
