# 0008 — Contexts reference each other by id through consumer-side interfaces

- Status: accepted
- Date: 2026-10-09

## Context

An invoice belongs to a customer. Importing the `customer` domain from
`invoice` would couple the two packages and make each harder to read and test
alone.

## Decision

- Entities reference others by id only (`customer_id`).
- The referencing domain declares the narrow interface it needs
  (`invoice.Customers{Exists}`); `cmd/server` passes the other context's
  store. Domain packages never import each other.
- A missing referenced entity is `FailedPrecondition`.

## Consequences

- Each domain is testable with a small fake.
- In memory, the existence check is not atomic with the insert, and deleting
  a customer does not check its invoices; a database enforces both with a
  foreign key.
