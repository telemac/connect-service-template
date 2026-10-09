# AGENTS.md

Go service boilerplate. Entities are defined once in Protobuf (`api/`) and
served over Connect, gRPC, gRPC-Web and REST from a single `connect.Server`
(connect-go v2). `docs/intent.md` explains why; `docs/patterns.md` details
every convention below. Read the relevant section of `docs/patterns.md`
before changing a contract.

## Commands

| Command | Does |
|---|---|
| `make setup` | Install TS deps from the lockfile (once after clone) |
| `make update-deps` | Move buf deps to their latest versions (commit `api/buf.lock`) |
| `make generate` | Regenerate Go, TS and OpenAPI from `api/` |
| `make check` | lint, generate, boundary check, vet, race tests, staticcheck, TS typecheck |
| `make breaking` | Fail on incompatible `api/` changes since `main` (`BREAKING_AGAINST=<ref>`) |
| `make run` | Start the server on `:8080`, with gRPC reflection |

Generated code (`gen/`, `ts/gen/`, `internal/openapi/openapi.yaml`) is not
committed. Run `make generate` before building. CI
(`.github/workflows/check.yml`) runs `make setup check` on a fresh clone,
`make breaking` against the pull request base, then fails if `go mod tidy` or
any tracked file would change. An intentional breaking change needs a new
package version (`<context>.v2`), not an edit of `v1`.

## Layout

| Path | Contents |
|---|---|
| `api/<context>/v1/*.proto` | Source of truth: entities, RPCs, validation, REST routes |
| `internal/<context>/` | Domain: plain Go types, invariants, operations |
| `internal/<context>/<context>rpc/` | Adapter: Connect handler, mappers, error mapping |
| `internal/pagetoken/` | Opaque List page tokens bound to their parameters |
| `internal/openapi/` | Embeds the generated OpenAPI document |
| `cmd/server/` | Wiring: builds dependencies, mounts every protocol |
| `ts/` | Generated TypeScript client package |
| `docs/` | Intent, patterns, decisions (ADRs) |

Examples to copy:

- `customer` — standard methods: Create, Get, List (filters, sort,
  pagination), partial Update with `update_mask`, Delete.
- `invoice` — references a customer, status state machine with custom
  methods (`:issue`, `:pay`, `:void`), timestamps, money; no Update/Delete.

## Rules

- Never edit generated code. Change the `.proto`, then `make generate`.
- Conventions are decisions: `docs/decisions/` (indexed in `docs/intent.md`
  §13) says why each exists. Changing one means a new ADR (next number,
  superseding the old one) and an update of `docs/patterns.md`.
- Domain packages import no `connectrpc.com/...`, no `gen/go/...`, no
  `net/http`, and no other domain package; they reference other contexts
  through a small interface they declare. `make boundaries` checks imports.
- Each RPC has its own `<Method>Request` / `<Method>Response`.
- Validation (patterns §4): field rules on the entity with
  `IGNORE_IF_ZERO_VALUE`; presence enforced on the request with CEL;
  invariants in the domain. Output-only fields are never read from input.
- Updates are AIP-134 partial updates (patterns §5). Status changes go
  through custom methods, never updates (patterns §7).
- Lists use typed filters, a typed sort enum and `internal/pagetoken`
  (patterns §6).
- Only adapters build `*connect.Error`; domain errors are sentinels mapped
  with `errors.Is` (patterns §9). Unknown errors are logged and returned as
  `Internal` with no message.
- Mapping is explicit (`toContract`, `toChange`, ...). No reflection.
- Browser access is off by default; enable it with
  `-cors-origins` (patterns §10). Never widen it in code. Same for gRPC
  reflection (`-reflection`); health checks are always on.
- Ids are `uuid.NewV7()`; money is `int64` minor units + ISO 4217 code; time
  is `time.Time` ↔ `google.protobuf.Timestamp`.
- Dependencies are passed through constructors and wired in `cmd/server`.
  No package-level mutable state, no `init()`.
- Logging: `log/slog` only, structured attributes, logger passed explicitly.
- Context is the first parameter of anything that does or may do I/O.
- Tests: table-driven, `assert := assert.New(t)`, `require` for setup,
  adapters tested through `connectinprocess` with the validate interceptor.

## Recipe: add an entity

1. Pick the closest example (`customer` for CRUD, `invoice` for a lifecycle
   or a reference) and copy its `.proto` to `api/<context>/v1/<context>.proto`
   (package `<context>.v1`, `go_package` under `gen/go/<context>/v1`).
2. Adapt the entity: field rules with `IGNORE_IF_ZERO_VALUE`, field
   behaviors, CEL presence rules on the requests, `google.api.http` routes,
   idempotency levels (patterns §3).
3. `make generate`.
4. Create `internal/<context>/`: domain type, sentinel errors, `Store`
   operations, `Filter`. Test it.
5. Create `internal/<context>/<context>rpc/`: handler, mappers, error mapping,
   list parameters bound to page tokens. Test it with `connectinprocess`.
6. Construct the store, register the handler and add the service name to the
   health checker in `newHandler` (`cmd/server/main.go`); add a REST case to
   `cmd/server/main_test.go`.
7. `make check`.

## Recipe: add an RPC

1. Add the RPC and its dedicated `<Method>Request` / `<Method>Response`
   messages. Never reuse a field number.
2. `make generate`; the compiler then points at the handler to complete.
3. Add the domain operation, then the handler method and any new error
   mapping. Test both.
4. `make check`.
