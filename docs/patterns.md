# Patterns

The detailed conventions behind `AGENTS.md`. Each section names the example
that implements it; read the code next to the rule.

- `customer` — standard methods: Create, Get, List (filters, sort, pagination),
  partial Update, Delete.
- `invoice` — a reference to another entity, a status state machine driven by
  custom methods, timestamps, money, no Update/Delete.

## 1. Request flow

```
HTTP request (Connect | gRPC | gRPC-Web | REST)
  → connecthttp.Mount / vanguard.Mount          cmd/server/main.go
  → connect.Server + validate interceptor       contract rules (protovalidate, CEL)
  → <context>rpc.Handler                        parse ids, map contract → domain
  → <context>.Store                             invariants, state, operations
  ← <context>rpc mappers + error mapping        domain → contract, error → code
```

Every protocol goes through the same `connect.Server`, so interceptors,
validation and handlers run identically for REST and RPC.

## 2. Layers and imports

| Package | May import | Must not import |
|---|---|---|
| `internal/<context>` (domain) | stdlib, other domain packages only through a consumer-side interface | `connectrpc.com/...`, `gen/go/...`, `net/http` |
| `internal/<context>/<context>rpc` (adapter) | its domain, generated code, connect, `internal/pagetoken` | other adapters |
| `internal/pagetoken` | stdlib | transport, domain |
| `cmd/server` | everything | — |

`make boundaries` fails if a domain package imports transport or generated
code. Only adapters build `*connect.Error`.

## 3. Contract conventions

- One file per context: `api/<context>/v1/<context>.proto`, package
  `<context>.v1`, `go_package` under `gen/go/<context>/v1`.
- Every RPC has its own `<Method>Request` / `<Method>Response`, even when the
  response only wraps the entity (keeps responses extensible; `buf lint`
  STANDARD enforces it).
- Field behaviors (`google.api.field_behavior`) document intent and drive
  OpenAPI: `REQUIRED`, `OUTPUT_ONLY` (`readOnly`), `IMMUTABLE`.
- Enums: zero value `<ENUM>_UNSPECIFIED`; request fields of enum type carry
  `enum.defined_only = true`.

| Method | HTTP | Idempotency | Request | Unknown id |
|---|---|---|---|---|
| Create | `POST /v1/<entities>`, `body: "<entity>"` | — | `<Entity> <entity>` | — |
| Get | `GET /v1/<entities>/{id}` | `NO_SIDE_EFFECTS` | `string id` | `NotFound` |
| List | `GET /v1/<entities>` | `NO_SIDE_EFFECTS` | see §6 | — |
| Update | `PATCH /v1/<entities>/{id}`, `body: "<entity>"` | `IDEMPOTENT` | see §5 | `NotFound` |
| Delete | `DELETE /v1/<entities>/{id}` | `IDEMPOTENT` | `string id` | `NotFound` |
| Custom | `POST /v1/<entities>/{id}:<verb>`, `body: "*"` | — | `string id` | `NotFound` |

## 4. Validation

Three layers, each with one job:

1. **Field rules on the entity message**, with `IGNORE_IF_ZERO_VALUE`: format
   and range (`string.uuid`, `string.email`, `string.pattern`, `max_len`,
   `int64.gt`). They apply whenever a field is set, in any request.
2. **Presence on the request message**, with message-level CEL. Whether a
   REQUIRED field must be set depends on the request: always on create, only
   when listed in the mask on update.

   ```proto
   option (buf.validate.message).cel = {
     id: "customer.name.required"
     message: "customer.name is required"
     expression: "this.customer.name != ''"
   };
   ```
3. **Domain invariants** in constructors and operations (`customer.build`,
   `invoice.Store.Create`). They hold for every caller, not only the API.

Rules 1 and 2 run in the interceptor before the handler, produce
`InvalidArgument` with `buf.validate.Violations` details, appear in OpenAPI,
and run client-side in TypeScript via `@bufbuild/protovalidate`.

Output-only fields (`id`, `status`, timestamps) are ignored on input: the
handler never reads them.

## 5. Updates (AIP-134)

```proto
message UpdateCustomerRequest {
  string id = 1;                                  // target, from the path
  Customer customer = 2 [(buf.validate.field).required = true];
  google.protobuf.FieldMask update_mask = 3 [(buf.validate.field).field_mask = {
    in: ["*", "id", "name", "email"]
  }];
}
```

- Omitted mask → every populated field. `*` → every mutable field.
- Output-only paths in the mask are ignored (AIP-203). Unknown paths are
  rejected by `field_mask.in`.
- A field in the mask must be set (CEL rule, see §4).
- A non-empty `customer.id` that differs from `id` is rejected (CEL), so a
  client bug cannot update the wrong entity.
- The adapter turns the mask into a domain `Change` struct of pointers
  (`nil` = keep); the store merges and validates under one lock, so
  concurrent partial updates do not lose fields.

Entities that are never edited (invoices) simply have no Update.

## 6. Lists

- `page_size`: `gte: 0`; 0 = server default 1000; values above 10000 are
  coerced to 10000 (AIP-158).
- `page_token` / `next_page_token`: opaque, empty on the last page.
- **Keyset pagination**: the cursor is the last (sort key, UUIDv7 id)
  returned. UUIDv7 ids sort by creation time, so pages stay stable while rows
  are added, without server-side state. With SQL this becomes
  `WHERE (key, id) > ($1, $2) ORDER BY key, id LIMIT n`.
- **Tokens** are built by `internal/pagetoken`: base64 JSON of the listing
  parameters (filters, order) plus the cursor. A token replayed with other
  parameters is rejected (`ErrMismatch` → `InvalidArgument`).
- **Filters** are typed request fields (`name_contains`, `email`,
  `customer_id`, `status`), validated like any other field and combined with
  AND. The domain receives a `Filter` struct whose zero value matches
  everything. No AIP-160 `filter` string (decision: typed fields are visible
  in OpenAPI and trivially map to SQL).
- **Sorting** (customer only) is a typed enum `order_by`
  (`<ENTITY>_ORDER_UNSPECIFIED` = creation order) plus `bool descending`; ties
  break on id so the order is total. No AIP-132 `order_by` string.

## 7. State machines and custom methods (AIP-136)

- The status is `OUTPUT_ONLY`; clients change it only through custom methods
  (`:issue`, `:pay`, `:void`), never through an update.
- Allowed transitions live in one place, `invoice.Status.canBecome`.
- A forbidden transition is `ErrTransition` → `FailedPrecondition`, and leaves
  the entity unchanged.

## 8. References between contexts

- Reference other entities by id only (`customer_id`).
- The referencing domain declares the narrow interface it needs
  (`invoice.Customers{Exists}`); `cmd/server` passes the other context's
  store. Domain packages never import each other directly, and tests use a
  fake.
- A missing referenced entity is `FailedPrecondition`, not `NotFound` (the
  invoice itself exists or is being created).
- The in-memory check is not atomic with the insert, and deleting a customer
  does not check its invoices. A database enforces both with a foreign key.

## 9. Errors

| Domain error | Connect code | HTTP (REST) |
|---|---|---|
| contract violation (interceptor) | `InvalidArgument` | 400 |
| `ErrInvalid` | `InvalidArgument` | 400 |
| `ErrNotFound` | `NotFound` | 404 |
| `ErrTransition`, `ErrUnknownCustomer` | `FailedPrecondition` | 400 |
| anything else | `Internal`, message hidden, logged with `slog` | 500 |

- Domain errors are sentinel values wrapped with context
  (`fmt.Errorf("%w: %s", ErrNotFound, id)`); adapters match them with
  `errors.Is` and attach the original with `WithCause`.
- connect-go v2 only sends messages of `*connect.Error` values built locally;
  anything else reaches the client as a bare code.
- REST errors use the `google.rpc.Status` shape (`{"code": 3, ...}`, numeric
  code), unlike the Connect protocol (`{"code": "invalid_argument"}`).

## 10. Types

- Ids: stdlib `uuid.UUID`, generated with `uuid.NewV7()` (time-ordered); the
  contract carries strings validated with `string.uuid`.
- Money: `int64` minor units (`amount_cents`) plus an ISO 4217 `currency`.
  Never floats.
- Time: `time.Time` in the domain, `google.protobuf.Timestamp` in the
  contract (`timestamppb.New`).

## 11. Tests

- Domain: table-driven, through the exported API, fakes for consumer-side
  interfaces.
- Adapter: through `connectinprocess` with the real validate interceptor, so
  contract rules are exercised without a network listener. A good check that
  validation rejected a request (not the handler): use an unknown id — the
  handler would answer `NotFound`, the interceptor answers `InvalidArgument`.
- Server: `cmd/server/main_test.go` covers Connect over HTTP (binary), REST
  routes, custom methods and the OpenAPI document.
- `assert := assert.New(t)` per test; `require` for setup that must not fail.

## 12. Tooling notes

- Generated code is not committed; `make generate` first, `make check` before
  committing.
- Dependencies come from lockfiles only: `make setup` runs `npm ci`, and buf
  reads `api/buf.lock` on demand. Upgrading is explicit (`make update-deps`,
  `npm update` in `ts/`, `go get`). CI pins the buf CLI version and fails on
  any drift (`go mod tidy -diff`, `git diff --exit-code`).
- Go plugins are pinned by the `tool` directive in `go.mod`
  (`go tool protoc-gen-go`); `protoc-gen-es` by `ts/package.json`.
- `buf` config lives in `api/`; outputs use `module=` with the full
  `gen/go` prefix.
- TS generation needs `import_extension=js` (NodeNext) and maps
  `buf/validate` to `@bufbuild/protovalidate`.
- The OpenAPI document is generated into `internal/openapi/` because
  `go:embed` only reads files inside the package directory.
- Vanguard is pinned to a `main` commit until it tags a release supporting
  connect-go v2.
- `connect-go-v2-migrate` (dry run by default) reports no rewrites on this
  project; its only warning flags any Vanguard import and is already
  satisfied by `vanguard.Mount`.
