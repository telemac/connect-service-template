# 0007 — Filters and sorting are typed request fields

- Status: accepted
- Date: 2026-10-08

## Context

AIP-160 (`string filter`) and AIP-132 (`string order_by`) are the Google
standards. `go.einride.tech/aip` (MIT, maintained, v0.x) parses and
type-checks AIP-160 filters but provides neither in-memory evaluation nor SQL
translation; the Go AIP-160 → SQL projects found had 0–10 stars.

## Decision

- Filters are typed fields on the List request (`name_contains`, `email`,
  `customer_id`, `status`), validated like any other field and combined with
  AND. The domain receives a `Filter` struct whose zero value matches
  everything.
- Sorting is a typed enum `order_by` (`<ENTITY>_ORDER_UNSPECIFIED` = creation
  order) plus `bool descending`; ties break on id.

## Consequences

- Filters appear as typed query parameters in OpenAPI and TypeScript, and map
  trivially to indexed SQL.
- No OR / NOT / parentheses; each new criterion is a contract change.
- An AIP-160 `filter` field can still be added later without breaking
  existing clients.
