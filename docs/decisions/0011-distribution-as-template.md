# 0011 — Distribute as a gonew template, not a library

- Status: proposed — preparation tracked in [docs/gonew.md](../gonew.md)
- Date: 2026-10-09

## Context

The project's value is its conventions, tooling and examples. Each project
needs its own contracts, generated code and domains; everything Go is under
`internal/`, so nothing is importable anyway.

## Decision

- New projects start from this repository with `gonew`, which copies it under
  a new module path.
- A shared Go module is extracted only once two real projects duplicate the
  same code, starting with `internal/pagetoken` and the server wiring.

## Consequences

- Improvements to the template do not reach existing projects automatically.
- The module path must disappear from non-Go files first (`.proto`
  `go_package`, `api/buf.gen.yaml`), since `gonew` only rewrites Go code.
