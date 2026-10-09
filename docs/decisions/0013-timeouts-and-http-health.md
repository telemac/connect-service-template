# 0013 — Unary calls are time-bounded; health is also served over plain HTTP

- Status: accepted (complements 0012)
- Date: 2026-10-09

## Context

The server only set `ReadHeaderTimeout`, so a slow dependency could hold a
request forever. `http.Server` `ReadTimeout`/`WriteTimeout` would bound
everything, including streams (reflection, health `Watch`), which must stay
open. Load balancers that only speak plain HTTP cannot call
`grpc.health.v1.Health`, even though it already answers Connect `POST` JSON.

## Decision

- A server interceptor gives every unary call a 30 s deadline, whatever the
  protocol (Connect, gRPC, gRPC-Web, REST). Streams are left unbounded.
- `http.Server` keeps `ReadHeaderTimeout` (10 s) and adds `IdleTimeout`
  (2 min) for idle keep-alive connections; no `ReadTimeout`/`WriteTimeout`.
- `GET /healthz` answers `200 ok` or `503`, from the same checker as
  `grpc.health.v1` (overall status).
- Message size keeps connecthttp's default read limit (4 MiB).

## Consequences

- The deadline only helps if handlers pass `ctx` to every I/O call (already a
  rule).
- `/healthz` is as static as the gRPC health service (ADR 0012).
