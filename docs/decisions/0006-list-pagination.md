# 0006 — Lists use AIP-158 keyset pagination with bound tokens

- Status: accepted
- Date: 2026-10-08

## Context

An "everything, unpaginated" mode was considered (`page_size = 0`), but an
AIP-158 client sending 0 expects a bounded default page and could receive an
unbounded response.

## Decision

- `page_size`: `gte: 0`; 0 = server default 1000; values above 10000 are
  coerced to 10000. No unpaginated mode: clients that want everything follow
  `next_page_token`.
- Keyset pagination: the cursor is the last (sort key, UUIDv7 id) returned.
  UUIDv7 ids are time-ordered, so pages stay stable while rows are added,
  with no server-side state.
- `internal/pagetoken` encodes tokens as opaque base64 JSON of the listing
  parameters (filters, order) plus the cursor. A token replayed with other
  parameters is rejected with `InvalidArgument`.

## Consequences

- Maps directly to SQL: `WHERE (key, id) > ($1, $2) ORDER BY key, id LIMIT n`.
- Token comparison is strict: `name_contains=ADA` then `ada` is rejected even
  though results match.
- No total count and no random page access.
