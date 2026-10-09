# 0004 — Every RPC has its own response message

- Status: accepted
- Date: 2026-10-08

## Context

`GetCustomer` could return `Customer` directly, as AIP-131 allows, and REST
clients would get a flatter body.

## Decision

Every RPC returns a dedicated `<Method>Response` (e.g.
`GetCustomerResponse { Customer customer = 1; }`), as `buf lint` STANDARD
requires. No `response_body` unwrapping in `google.api.http`.

## Consequences

- Response-level fields can be added later without a breaking change.
- One rule for agents: one Request/Response pair per RPC, no lint exceptions.
- REST bodies are wrapped (`{"customer": {...}}`).
