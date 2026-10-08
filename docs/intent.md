# Intent

Status: accepted (2026-10-08) — option A, proto-first with Connect v2 (see [Decision](#6-decision)).

## 1. Purpose

This repository is a starting template for Go services. Its job is to frame how
entities are defined and how they move through a service. Every entity has
**one source of truth** and **one defined path** from the wire to the domain and
back. The rules are explicit enough that a human or an AI agent can add a new
entity or endpoint without guessing.

## 2. Goals

- **Single source of truth per entity.** Everything else is generated or
  mechanically derived from it.
- **Clear layering.** Contract types (API) and domain types (business) are
  separate. Explicit mappers connect them.
- **Validation lives with the definition.** Constraints are declared next to the
  type and enforced at the boundary.
- **One definition, every consumer.** The same definitions serve:
  - TypeScript / web frontends,
  - internal Go services,
  - public third-party REST clients (with an OpenAPI document),
  - AI agents and MCP tools (through machine-readable schemas).
- **AI-friendly.** Predictable layout, one way to do each thing, written
  recipes, and fast deterministic checks.
- **Minimal root.** The repository root holds only what tools require there.

## 3. Non-goals (v1)

- Persistence and database mapping.
- Authentication and authorization.
- Deployment, containers, infrastructure.
- Observability stack (metrics, tracing).
- Multi-service orchestration.
- A frontend application.

These may come later. They must not shape v1.

## 4. Glossary

| Term | Meaning |
|---|---|
| **Entity** | A business concept with identity (e.g. `User`, `Order`). |
| **Contract type** | The wire representation of an entity, as exposed by the API. |
| **Domain type** | The plain Go representation used by business logic. |
| **Mapper** | A hand-written function converting between a contract type and a domain type. |
| **Boundary** | The handler layer, where contract types are validated and mapped. |
| **Source of truth** | The single file where an entity's shape and constraints are declared. |
| **Generated code** | Code produced by a tool from the source of truth. Never edited by hand. |

## 5. Options for the source of truth

### A. Proto-first with Connect v2

Entities are declared in `.proto` files, with constraints expressed through
[protovalidate](https://github.com/bufbuild/protovalidate). `buf` generates Go
code and TypeScript clients (Connect-ES). A single handler serves the Connect,
gRPC and gRPC-Web protocols. REST is exposed through `google.api.http`
transcoding, and an OpenAPI document is generated from the protos.

Connect v2 (`connectrpc.com/connect/v2`) is relevant because:

- handlers take and return plain messages (no `Request[T]` / `Response[T]` wrappers);
- the core is transport-agnostic: `connecthttp` for `net/http`, `connectinprocess`
  for network-free tests;
- only an explicit `*connect.Error` reaches the client; `WithCause` keeps the
  internal cause for `errors.Is` / `errors.As`;
- the exported API is small (~25 identifiers), which keeps it easy to learn for
  humans and agents.

### B. Go-first with Huma v2

Entities are Go structs with tags. [Huma v2](https://github.com/danielgtaylor/huma)
(`github.com/danielgtaylor/huma/v2`) generates OpenAPI 3.1 and JSON Schema at
runtime, validates input automatically and returns RFC 9457 problem errors. It
runs on the standard library mux or chi. TypeScript clients are generated from
the OpenAPI document.

### C. Hybrid

Proto for internal RPC, Huma for public REST. This requires two contract sets
for the same entity, with mappers on both sides.

### Trade-offs

| | A. Proto + Connect v2 | B. Go + Huma v2 | C. Hybrid |
|---|---|---|---|
| Single source of truth | Yes (`.proto`) | Yes (Go structs) | **No** |
| TS / web client | Native, typed (Connect-ES) | Via OpenAPI generator | Both |
| Internal Go services | Native, typed client | HTTP + generated client | Native |
| Public REST + OpenAPI | Via transcoding + generator | **Native, best-in-class** | Native |
| AI agents / schemas | Proto descriptors + OpenAPI | OpenAPI / JSON Schema | Both |
| Breaking-change detection | `buf breaking` | Manual / OpenAPI diff | Partial |
| Toolchain | `buf` + codegen step | Go only | Both |
| Go ergonomics | Generated proto types | Idiomatic structs | Mixed |

Option C is rejected unless forced: it duplicates entity definitions, which
contradicts the core goal.

## 6. Decision

**Option A — proto-first with Connect v2**, accepted on 2026-10-08. The
ecosystem check in §11 confirms it is viable.

Rationale: three of the four consumers (TS frontends, Go services, AI agents)
benefit most from a language-neutral contract with typed clients and
breaking-change detection. Connect v2's plain-message handlers and in-process
transport remove most of the ergonomic cost v1 had. Public REST is covered by
transcoding and a generated OpenAPI document.

**Fallback:** if public REST becomes the dominant consumer, or if the REST
transcoding path is not viable with Connect v2, switch to option B (Huma).

This decision is recorded as the first ADR in `docs/decisions/`.

## 7. Rules for entities

1. **Contract types are generated.** Never hand-edited. Never used outside the
   boundary.
2. **Domain types are plain Go.** No imports from proto, Connect or any API
   framework.
3. **Mapping is explicit.** Hand-written `toDomain` / `toContract` functions,
   covered by tests. No reflection-based mappers.
4. **Validation at the boundary.** Declared in the source of truth, enforced
   before the domain is reached. The domain still guards its own invariants.
5. **Errors are mapped at the boundary.** Domain errors are plain Go errors;
   only the handler turns them into API errors.
6. **One package per bounded context.** Each package must fit in a developer's
   head.
7. **Make invalid states unrepresentable.** Typed enums, validating
   constructors, and useful zero values in domain types.

### Go code rules

Taken from the `go` bundle of the global OKF workspace (`~/.okf/go`):

- **Packages by domain, not by technical layer.** `internal/order`, not
  `internal/handlers`. No `utils/`, `common/` or `helpers/` packages. The
  package name is part of the API (`order.Cancel()`).
- **Minimal exported surface.** `go doc` of a package fits on one screen.
- **Dependencies point inward.** Domain code imports no infrastructure: no
  `net/http`, no Connect, no generated proto. Adapters (handlers) depend on the
  domain, never the reverse. This is greppable and checked.
- **Explicit dependencies.** Collaborators are passed through constructors.
  No package-level mutable state, no `init()` wiring. Composition happens in
  `cmd/server/main.go`; consumer-side interfaces stay small.
- **Rigor proportional to criticality.** A CRUD context stays flat; the
  domain/handler split is applied only where business logic warrants it.
- **Errors.** Use `errors.AsType` (Go 1.26+) rather than `errors.As` in new code.
- **Logging.** Use `log/slog` only, with structured key/value attributes. The
  `*slog.Logger` is built in `main` and passed explicitly; no use of the
  package-level default logger in library code.

## 8. Target repository layout

The root only holds files that tools require there:

```
go.mod
go.sum
README.md
AGENTS.md
CLAUDE.md            # one line pointing to AGENTS.md
.gitignore
Makefile
api/                 # buf.yaml, buf.gen.yaml, buf.lock, <package>/v1/*.proto
gen/                 # generated Go + OpenAPI — not committed
ts/                  # self-contained TS client: package.json, tsconfig.json, gen/ (not committed)
cmd/server/          # entrypoint and wiring
internal/<context>/  # domain types, service, mappers, handler
docs/                # intent.md, decisions/ (ADRs)
```

Tool configuration lives next to what it configures: `buf` config under
`api/`, npm manifests under `ts/`.

Generated code is never committed. `make generate` must run before building;
CI does the same.

## 9. AI-friendliness requirements

- `AGENTS.md` at the root is the single entry point for agents. `CLAUDE.md`,
  if present, only points to it.
- Step-by-step recipes: *add an entity*, *add an RPC*, *add a validation rule*.
- One canonical example entity implemented end to end, to be copied.
- One command regenerates code; one command runs every check.
- Decisions are recorded as short ADRs in `docs/decisions/`.
- Conventions are enforced by checks, not only by documentation.

## 10. Definition of done (v1)

- One example entity exposed through Connect (JSON and binary).
- Called from a Go test through `connectinprocess`, without a network listener.
- Reachable as REST, with a served OpenAPI document.
- Invalid input returns a structured validation error.
- `make check` passes from a fresh clone: `buf lint`, generate, `go vet ./...`,
  `go test ./...`, TS typecheck.

## 11. Ecosystem check (2026-10-08)

Connect v2.0.0 was released on 2026-10-07. Status of the pieces option A needs:

| Piece | Module | Connect v2 status | Risk |
|---|---|---|---|
| REST transcoding | `connectrpc.com/vanguard` | Rewritten for v2 on `main` (PR #203, merged 2026-10-07): `vanguard.Mount(mux, server)` serves `google.api.http` routes from the same `connect.Server`. **Not released yet**; latest tag v0.4.0 targets v1, `main` still pins `v2.0.0-rc.1`. | Medium — pin a commit until a release is tagged. |
| OpenAPI generator | `github.com/sudorandom/protoc-gen-connect-openapi` | v0.28.0 (2026-09-30), depends on `connect/v2 v2.0.0`. Handles `google.api.http` and protovalidate constraints. Actively maintained. | Low |
| OpenAPI generator (alt) | `github.com/google/gnostic` | Last release v0.7.0 (2023). | Rejected — stale. |
| Validation interceptor | `connectrpc.com/validate` | v0.8.0 (2026-10-07), depends on `connect/v2 v2.0.0` and `protovalidate v1.4.0`. | Low |
| TS client | Connect-ES | Wire protocol unchanged; independent of the Go major version. | Low |

Conclusion: option A is viable with Connect v2. The only gap is that Vanguard
has no v2 release yet, so the boilerplate pins a `main` commit and tracks the
next tag. This does not justify falling back to Huma.

## 12. Resolved questions

| Question | Decision | Why |
|---|---|---|
| Commit `gen/`? | No — generated locally and in CI | Keeps diffs on the contract only; generation is one command. |
| Minimum Go version | 1.27 | Standard `uuid` package (v7 IDs), `errors.AsType`; Vanguard `main` already requires 1.26. |
| `Makefile` or `justfile` | `Makefile` | No extra tool to install. |
| TS generation location | Self-contained `ts/` | Keeps npm files out of the root. |

## 13. References

- Global OKF workspace (`~/.okf`), `go` bundle — principles applied in §7.
- Ecosystem facts in §11 were checked directly against the upstream
  repositories on 2026-10-08.
