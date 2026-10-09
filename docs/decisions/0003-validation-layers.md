# 0003 — Validation is split between entity, request and domain

- Status: accepted
- Date: 2026-10-08

## Context

Constraints should be declared once, next to the entity, and be visible to
every consumer (OpenAPI, TypeScript, AI agents). But whether a field must be
present depends on the request: always on create, only when listed in the
mask on a partial update (ADR 0005). Rules such as `min_len: 1` on the entity
would reject every partial update.

## Decision

1. Field rules (format, range, length) live on the entity message with
   `IGNORE_IF_ZERO_VALUE`: they apply whenever a field is set.
2. Presence is enforced on the request message with message-level CEL rules,
   and documented on the entity with `(google.api.field_behavior) = REQUIRED`.
3. The domain still guards its own invariants in constructors and operations.

Rules 1 and 2 run in the `connectrpc.com/validate` interceptor, before the
handler, for every protocol including REST.

## Consequences

- Invalid requests get `InvalidArgument` with `buf.validate.Violations`
  details; the same rules run client-side with `@bufbuild/protovalidate`.
- Each request with required fields carries a few CEL rules; they are the
  price of reusing one entity message across create and update.
- Some checks exist twice (contract and domain) on purpose: the domain cannot
  trust that every caller went through the API.
