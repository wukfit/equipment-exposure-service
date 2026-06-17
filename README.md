# Equipment Exposure Service

An HTTP API in Go for tracking worker exposure to **Hand & Arm Vibration
Syndrome (HAVS)**. Clients record *exposures* — a user operating a piece of
equipment for a duration — and retrieve a user's aggregated exposure over a time
window. Two metrics matter, per the HSE vibration model: **Partial Exposure
Points** and **Partial Exposure A(8)**.

The API contract is [`spec.yaml`](./spec.yaml) (OpenAPI 3.0.3). The full design —
architecture, domain model, resolved spec decisions, and testing strategy — is in
[`docs/design.md`](./docs/design.md).

Built with Domain-Driven Design and a clean ports-&-adapters architecture,
comprehensive tests across every layer, and single-command runnability.

## Quick start

### Run

```bash
docker compose up --build        # builds and serves on :8080 (in-memory store)
# …or without Docker:
make run                         # go run ./cmd/server
```

Configuration is env-driven: `PORT` (default `8080`) and `LOG_LEVEL`
(`debug|info|warn|error`, default `info`). No database, secrets, or auth.

```bash
curl -s localhost:8080/health    # {"status":"ok"}
```

### Test

```bash
make test                        # go test -race -cover ./...
make generate                    # regenerate the API package from spec.yaml
```

## Seeded data

The in-memory catalog is seeded at startup with fixed, documented fixtures. The
IDs reuse the spec's own examples, so they are real, working IDs you can paste
straight into requests:

| Kind | ID | Detail |
|---|---|---|
| Equipment | `2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49` | AirCat – Drill – 4337 — **2.1 m/s²** |
| Equipment | `36603447-2f30-41b1-a908-526c0b6f1755` | JCB – Hydraulic Breaker – CEJCBHM25 — **4.0 m/s²** |
| User | `713be58e-0d79-4df2-a85c-9f44ca513a7d` | Bobby Tables |
| User | `a52a3c1e-7b2d-4c9a-9f0e-1d6b8c4f2a10` | Alice Stone |

## API

All four endpoints are defined in `spec.yaml`. Exposure responses embed the
associated `user` and `equipment`; the summary response embeds `user` only.
Errors share one `{ "error": "<slug>", "message": "…" }` shape.

### `POST /exposure` — record an exposure → **201**

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

### `GET /exposure/{id}` — fetch one → **200**

```bash
curl -s localhost:8080/exposure/<id-from-the-POST>
```

### `GET /exposure` — list all → **200** (ordered by recorded time, then id)

```bash
curl -s localhost:8080/exposure
```

### `GET /users/{userId}/exposure-summary` — aggregate over a window → **200**

```bash
# default window = trailing 24h [now-24h, now)
curl -s localhost:8080/users/713be58e-0d79-4df2-a85c-9f44ca513a7d/exposure-summary

# explicit half-open window [starting_at, ending_at) — RFC3339, UTC-normalised
curl -s "localhost:8080/users/713be58e-0d79-4df2-a85c-9f44ca513a7d/exposure-summary?starting_at=2025-01-01T00:00:00Z&ending_at=2025-12-31T23:59:59Z"
```

A user with two in-window exposures (AirCat 30 min + JCB 2 h) summarises to:
```json
{"a8":2.0677586,"points":68,"user":{"id":"713be58e-…","name":"Bobby Tables"}}
```

> Exposures are stamped with the server's current time, so the **window must
> contain "now"** to include freshly-recorded exposures. The default trailing-24h
> window does; a window that lies entirely in the past — e.g. the explicit 2025
> example above, on a server running later — returns `{"a8":0,"points":0,…}`.

**Try it end-to-end** — record an AirCat (30 min) and a JCB (2 h) for Bobby, then
summarise with the default window (both land in the trailing-24h window):

```bash
BOB=713be58e-0d79-4df2-a85c-9f44ca513a7d
curl -s -XPOST localhost:8080/exposure -H 'Content-Type: application/json' \
  -d "{\"user_id\":\"$BOB\",\"equipment_id\":\"2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49\",\"duration\":30}" >/dev/null
curl -s -XPOST localhost:8080/exposure -H 'Content-Type: application/json' \
  -d "{\"user_id\":\"$BOB\",\"equipment_id\":\"36603447-2f30-41b1-a908-526c0b6f1755\",\"duration\":120}" >/dev/null
curl -s localhost:8080/users/$BOB/exposure-summary    # → a8 ≈ 2.0678, points 68
```

### Error contract

| Status | When | Example slug |
|---|---|---|
| `400` | malformed JSON, missing/wrong-typed/trailing body field, bad UUID or date param, `starting_at` after `ending_at` | `invalid_request`, `invalid_window` |
| `404` | unknown user / equipment / exposure | `user_not_found`, `equipment_not_found`, `exposure_not_found` |
| `422` | well-formed but invalid value (e.g. `duration: 0`) | `invalid_duration` |
| `500` | unexpected / referential data-consistency fault (generic body; detail logged) | `server_error` |

## The HAVS calculation

Per exposure, from a vibration magnitude `m` (m/s²) and duration `t` (minutes,
`h = t/60`):

```
a8     = m · √(h / 8)
points = round( (m / 2.5)² · (h / 8) · 100 )
```

Aggregating a user's exposures over a window:

```
combined A(8) = √Σ a8ᵢ²     (root-sum-of-squares — vibration energies add)
combined pts  = Σ pointsᵢ    (HSE points are additive)
```

A(8) values are combined from the **unrounded** per-exposure `a8`; points are the
sum of the already-rounded per-activity points. An empty window yields zeros.

> **Why RSS for A(8)?** A(8) is an RMS acceleration (an energy-like quantity), so
> doses combine in quadrature, not by plain addition. Plain-summing A(8) would
> overstate the dose and is physically wrong.

## Documented decisions

`spec.yaml` is the single source of truth: the corrections, additions, and
constraints below are applied directly to it, and the implementation, codegen,
and response-validation all agree with it.

| Decision | Rationale |
|---|---|
| **Corrected float maths** | The brief's reference code used Go *integer* division `(t/60)/8`, truncating to **0 for any exposure under 8 hours** — the API would return zeros for virtually all real input. Corrected to the mathematically intended float form. |
| **RSS for A(8); linear sum for points** | HSE-correct energy combination (see above). The spec was silent → documented assumption. |
| **`GET /exposure/{id}` → 200** (spec corrected from `201`) | 201 *Created* is wrong for fetching an existing resource. |
| **Window params `date-time`** (spec corrected from `date`) | The spec's own examples are RFC3339 instants; a calendar date can't express a window boundary. Normalised to UTC. |
| **Half-open window `[start, end)`** | An instant exactly on a boundary isn't double-counted across adjacent windows. |
| **Default window** | both omitted → trailing 24h `[now-24h, now)`; `start` only → `end=now`; `end` only → `start=end-24h`; `start > end` → 400. |
| **Internal `recordedAt` = server `now()`** | The response/POST schemas carry no timestamp, but the summary filters by a window, so one is required internally (injected `Clock`, not serialised). |
| **`ErrorResponse` schema + `required` lists added** | The supplied spec declared only success responses with no required fields. Adding the error contract *and* required fields makes the whole surface — success **and** error — provably validated against the spec in tests. |
| **POST body has no `required` list** (deliberate) | Required-ness is enforced at the boundary so an absent field is a clean **400**, kept distinct from **422** for a present-but-invalid value. |
| **OpenAPI 3.0.3** (not 3.1) | `oapi-codegen` does not yet support 3.1; the spec uses no 3.1-only features, so 3.0.3 keeps codegen and validation warning-free. |
| **Referential miss on a read → 500** | A stored exposure whose user/equipment can't be resolved is a server-side data-consistency fault, not a client 404. (Unreachable today with the immutable seed catalog; correct once the catalog becomes mutable.) |

## Architecture

Ports & adapters / DDD. The dependency rule is strict: `adapters → app → domain`,
and `domain` depends on nothing. The app layer never imports the generated HTTP
types. Ports are defined in the **application layer** (consumer-side, beside the
use cases that own them), not in the domain — persistence and event publication
are application concerns, so the domain stays pure.

```
cmd/server                              wiring, config, graceful shutdown
internal/domain                         Exposure aggregate, User, EquipmentItem,
                                        PartialExposure VO (HAVS maths + Aggregate),
                                        ExposureRecorded event, sentinels
internal/app                            application ports (ExposureStore, UserDirectory,
                                        EquipmentCatalog, EventPublisher), Clock, window
internal/app/command · query            RecordExposure; GetExposure, ListExposures,
                                        GetUserExposureSummary (+ ExposureReadModel)
internal/adapters/http                  oapi-codegen server, request→cmd/query and
                                        read-model→response mapping, error mapping,
                                        middleware
internal/adapters/repository/memory     in-memory adapters (behind the ports)
internal/adapters/events                slog EventPublisher adapter
internal/seed                           fixed seed fixtures (shared by app + tests)
internal/contracttest                   shared repository contract suite
```

## Events (event-driven architecture)

An in-code seam is included and exercised; a real broker is described, not built.

- **Publish:** `RecordExposure` emits `ExposureRecorded` after a successful save,
  via the `EventPublisher` port (a `slog` adapter logs it as structured JSON; a
  fake publisher asserts emission in tests). Natural downstream consumers derive
  `ExposureActionValueReached` / `ExposureLimitExceeded` against the HSE **EAV**
  (100 pts / A(8) 2.5) and **ELV** (400 pts / A(8) 5.0) — the alerting use case.
- **Consume:** `EquipmentRegistered`/`EquipmentUpdated` and
  `UserCreated`/`UserDeactivated` from other bounded contexts. This is *why* the
  service owns no user/equipment CRUD: that data is owned elsewhere and would
  arrive via events.

## Testing

Comprehensive, outside-in, test-first across four tiers (see design §10):
domain unit tests with pinned HSE reference values; exhaustive blackbox API
contract tests (`httptest`) covering every status branch; a shared repository
**contract suite** (run under `-race`, incl. 50-goroutine concurrency and the
half-open boundary) that any adapter must satisfy; and mapper/error-mapping unit
tests. Every blackbox response is validated against `spec.yaml` with
`kin-openapi` — body and status, success and error.

`go test -race -cover ./...` — coverage of the hand-written packages:

| Package | Coverage |
|---|---|
| `internal/domain` | 84.4% |
| `internal/app` | 90.9% |
| `internal/app/command` | 92.9% |
| `internal/app/query` | 94.1% |
| `internal/adapters/http` | 94.2% |
| `internal/adapters/repository/memory` | 91.8% |
| `internal/adapters/events` · `internal/seed` | 100% |

Generated code (`api.gen.go`) and `main.go` wiring are excluded from meaningful
coverage, so the aggregate `total` figure understates the tested surface.

## Compromises & next steps

- **Persistence is in-memory.** The `app.ExposureStore` port makes Postgres a
  drop-in adapter; the first next step is `pgx` + `sqlc` + `goose`, a compose
  `app + db`, and the **same** `contracttest` suite run via testcontainers —
  proving substitutability with no new test code.
- **`a8` / `points` serialise as `float32`** (the spec's `type: number`). All
  computation is `float64`; only the response boundary narrows, and JSON renders
  clean shortest-round-trip values. Switching to `float64` on the wire is a
  one-line `format: double` spec change if exact wire precision is wanted.
- The **corrected formula** and the **RSS aggregation** are documented assumptions
  to confirm with the domain owner.
