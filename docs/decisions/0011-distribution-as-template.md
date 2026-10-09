# 0011 — Distribute as a gonew template, not a library

- Status: accepted
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

- The module is `github.com/telemac/connect-service-template` (MIT).
- `gonew` rewrites only `go.mod` and Go imports, so the module path appears
  in a single non-Go place: buf managed mode's `go_package_prefix` in
  `api/buf.gen.yaml` (the `.proto` files carry no `go_package`; only
  `go_package` is managed, so generated code is unchanged). `make rename`
  rewrites it from `go list -m`, and `make generate` fails while they
  disagree.

## Consequences

- Starting a project: `gonew github.com/telemac/connect-service-template <module>`,
  then `make rename setup check`. Verified on 2026-10-09 with the real
  `gonew` against a local module proxy (`GOPROXY=file://`): the new project
  passes `make check` with no manual edit.
- `gonew` needs a published, tagged module; with `GOPRIVATE` covering the
  module path, Go fetches it directly from git instead of the proxy.
- Improvements to the template do not reach existing projects automatically.
