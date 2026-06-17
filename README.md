# Equipment Exposure Service

An HTTP API in Go for tracking worker exposure to Hand & Arm Vibration Syndrome
(HAVS). Clients record *exposures* — a user operating a piece of equipment for a
duration — and retrieve a user's aggregated exposure (Partial Exposure **Points**
and **A(8)**) over a time window.

The API contract is defined in [`spec.yaml`](./spec.yaml) (OpenAPI 3.1).

## Status

Under active development, delivered in small, independently reviewable slices
(one PR each). See the full design — architecture, domain model, resolved spec
decisions, testing strategy, and event-driven-architecture notes — in
[`docs/design.md`](./docs/design.md).

Run instructions, documented decisions, and example requests will land with the
finalisation slice.

## Tech

Go · stdlib `net/http` · oapi-codegen · in-memory persistence behind a
repository port (Postgres as a documented next step) · ports & adapters / DDD ·
comprehensive outside-in tests.
