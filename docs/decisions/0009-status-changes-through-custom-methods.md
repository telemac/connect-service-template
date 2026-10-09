# 0009 — Status changes go through custom methods

- Status: accepted
- Date: 2026-10-09

## Context

An invoice has a lifecycle (DRAFT → ISSUED → PAID, DRAFT/ISSUED → VOID).
Letting clients set `status` through an update would scatter transition rules
and make illegal jumps easy.

## Decision

- `status` is `OUTPUT_ONLY`.
- Each transition is an AIP-136 custom method:
  `POST /v1/<entities>/{id}:<verb>` (`:issue`, `:pay`, `:void`).
- Allowed transitions live in one domain function (`Status.canBecome`); a
  forbidden one is `FailedPrecondition` and leaves the entity unchanged.
- Entities that are never edited after creation have no Update or Delete.

## Consequences

- The API reads as business actions; the transition table is the single
  place to review.
- Each new transition is a new RPC.
