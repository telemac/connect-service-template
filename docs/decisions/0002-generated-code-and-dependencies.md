# 0002 — Generated code is not committed; dependencies come from lockfiles

- Status: accepted
- Date: 2026-10-08, extended 2026-10-09

## Context

Committing generated code doubles every contract change in review and lets
generated files drift from the `.proto`. Without it, every clone must be able
to regenerate deterministically.

## Decision

- `gen/`, `ts/gen/` and `internal/openapi/openapi.yaml` are git-ignored;
  `make generate` rebuilds them and `make check` always regenerates first.
- Generators are pinned: Go plugins through the `tool` directive in `go.mod`,
  `protoc-gen-es` through `ts/package-lock.json`, the buf CLI version in CI.
- `make setup` installs from lockfiles only (`npm ci`; buf reads
  `api/buf.lock` on demand). Upgrades are explicit (`make update-deps`,
  `npm update`, `go get`).
- CI runs `make setup check` on a fresh clone, then fails on
  `go mod tidy -diff` or `git diff --exit-code`.

## Consequences

- A build always needs `buf` and Node.js, even for Go-only work.
- Reviews show contract changes only.
- Dependency upgrades are visible commits, never a side effect of setup.
