# 0010 — CORS is opt-in

- Status: accepted
- Date: 2026-10-09

## Context

Web frontends on another origin cannot call the API without CORS, but an
open policy by default would expose every endpoint to any site.

## Decision

- Off by default; `cmd/server -cors-origins=<origin>,...` (or `*`) enables it
  for every protocol and the OpenAPI document.
- Methods and headers come from `connectrpc.com/cors`, plus `PATCH` and
  `DELETE` for REST; served by `github.com/rs/cors`.
- Credentials are not allowed.

## Consequences

- Revisit credentials and allowed headers (`Authorization`) when
  authentication is added.
- Preflight header lists must be sorted and lowercase, as browsers send them.
