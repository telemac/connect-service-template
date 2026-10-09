# 0005 — Updates are AIP-134 partial updates

- Status: accepted (supersedes the initial full-replacement `PUT`)
- Date: 2026-10-08

## Context

`UpdateCustomer` first replaced every field (`PUT`). Clients that want to
change one field then have to read, merge and resend the whole entity, and
concurrent writers overwrite each other.

## Decision

- `Update<Entity>Request { string id; <Entity> <entity>; google.protobuf.FieldMask update_mask; }`,
  REST `PATCH /v1/<entities>/{id}`, `IDEMPOTENT`.
- Omitted mask = every populated field; `*` = every mutable field;
  output-only paths are ignored (AIP-203); unknown paths are rejected by
  `field_mask.in`; a field named in the mask must be set (CEL, ADR 0003).
- `id` in the path is the target; a non-empty body id that differs is
  rejected (CEL), so a client bug cannot update another entity.
- The domain receives a `Change` struct of pointers (`nil` = keep) and merges
  under one lock.

## Consequences

- `PUT` no longer exists (405); full replacement is `PATCH` with
  `update_mask=*`.
- A field cannot be cleared to its zero value without naming it in the mask,
  and required fields cannot be cleared at all.
- Entities that are never edited (invoices) simply have no Update.
