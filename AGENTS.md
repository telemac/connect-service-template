# AGENTS.md

Go service boilerplate. Entities are defined once in Protobuf (`api/`) and
served over Connect, gRPC, gRPC-Web and REST from a single `connect.Server`
(connect-go v2). Read `docs/intent.md` for the why; this file is the how.

## Commands

| Command | Does |
|---|---|
| `make setup` | Install TS deps, resolve buf deps (once after clone) |
| `make generate` | Regenerate Go, TS and OpenAPI from `api/` |
| `make check` | lint, generate, boundary check, vet, race tests, TS typecheck |
| `make run` | Start the server on `:8080` |

Generated code (`gen/`, `ts/gen/`, `internal/openapi/openapi.yaml`) is not
committed. Run `make generate` before building.

## Layout

| Path | Contents |
|---|---|
| `api/<context>/v1/*.proto` | Source of truth: entities, RPCs, validation, REST routes |
| `internal/<context>/` | Domain: plain Go types, invariants, operations |
| `internal/<context>/<context>rpc/` | Adapter: Connect handler + contract ↔ domain mappers |
| `cmd/server/` | Wiring: builds dependencies, mounts every protocol |
| `ts/` | Generated TypeScript client package |
| `docs/` | Intent and decisions (ADRs) |

`customer` is the canonical example. Copy it.

## Rules

- Never edit generated code. Change the `.proto`, then `make generate`.
- Domain packages import no `connectrpc.com/...`, no `gen/go/...`, no
  `net/http`. `make boundaries` enforces it.
- Structural validation goes in the `.proto` (protovalidate), on the entity
  message, not on requests. Requests embed the entity with
  `(buf.validate.field).required = true`. Server-assigned fields are
  `OUTPUT_ONLY` + `IGNORE_IF_ZERO_VALUE`, and handlers ignore their input value.
  Domain constructors still guard invariants.
- Updates are full replacements: `Update<Entity>Request { string id; <Entity>
  <entity>; }`, REST `PUT /v1/<entities>/{id}`, `IDEMPOTENT`. The path id is
  the target; a message-level CEL rule rejects a non-empty body id that
  differs from it. No `update_mask` until an entity needs partial updates.
- Deletes: `DELETE /v1/<entities>/{id}`, `IDEMPOTENT`, empty
  `Delete<Entity>Response`. An unknown id is `NotFound`.
- Only the `*rpc` adapter builds `*connect.Error`. Unknown errors are logged
  and returned as `CodeInternal` with no message.
- Mapping is explicit (`toContract`, ...). No reflection-based mappers.
- Dependencies are passed through constructors and wired in `cmd/server`.
  No package-level mutable state, no `init()`.
- Logging: `log/slog` only, structured attributes, logger passed explicitly.
- Context is the first parameter of anything that does or may do I/O.
- Tests: table-driven, `assert := assert.New(t)`, Connect handlers tested
  through `connectinprocess`.

## Recipe: add an entity

1. Create `api/<context>/v1/<context>.proto` from `api/customer/v1/customer.proto`
   (package `<context>.v1`, `go_package` under `gen/go/<context>/v1`).
2. Put validation rules on the entity message; `Create<Entity>Request` embeds
   it (REST `body: "<entity>"`). Add `google.api.http` routes. Mark read-only
   RPCs `NO_SIDE_EFFECTS`.
3. `make generate`.
4. Create `internal/<context>/` with the domain type, constructor, sentinel
   errors and operations. Test it.
5. Create `internal/<context>/<context>rpc/` with the handler, mappers and
   error mapping. Test it with `connectinprocess`.
6. Register the handler in `newHandler` in `cmd/server/main.go`.
7. `make check`.

## Recipe: add an RPC

1. Add the RPC and its dedicated `<Method>Request` / `<Method>Response`
   messages to the existing `.proto`. Never reuse a field number.
2. `make generate`; the compiler then points at the handler to complete.
3. Add the domain operation, then the handler method. Test both.
4. `make check`.
