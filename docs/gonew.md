# Preparing the project for gonew

Status: todo — not started.

Goal: start new projects from this one with
[`gonew`](https://pkg.go.dev/golang.org/x/tools/cmd/gonew), which downloads a
module and copies it under a new module path.

`gonew` only rewrites the module path in `go.mod` and in `.go` import paths.
Every other file is copied unchanged.

## 1. Module path outside Go files

The blocking issue. The path appears in:

- `option go_package` in every `.proto` (`api/customer/v1/customer.proto`,
  `api/invoice/v1/invoice.proto`);
- the two `opt: module=github.com/telemac/connectrpc-v2-poc/gen/go` in
  `api/buf.gen.yaml`.

After `gonew`, `make generate` would emit code under the old path and the
build would break.

Plan:

- Enable buf managed mode (`go_package_prefix`) and remove `go_package` from
  the `.proto` files, so the path only remains in `api/buf.gen.yaml`.
- Add a `make rename` target that reads the module path with `go list -m` and
  rewrites it in `api/buf.gen.yaml`.

## 2. Publish the module

`gonew` fetches through the Go module proxy (like `go mod download`).

- Push the repository (see §4 for the name).
- Tag a semver version (e.g. `v0.1.0`) so `@latest` resolves to a stable
  version.
- Private repository: users need `GOPRIVATE=github.com/telemac/*`.

## 3. Module archive contents

- Only git-tracked files are included. `gen/`, `ts/gen/` and
  `internal/openapi/openapi.yaml` are ignored, hence absent — intended; users
  run `make setup && make check`.
- No nested `go.mod` (true today: `ts/` has none).

## 4. Names gonew does not rewrite

- The project name: `README.md` title, `ts/package.json` `name`, mentions in
  `docs/`. Pick a neutral template name now (e.g. `go-service-template`).
- Proto packages (`customer.v1`, `invoice.v1`) stay valid as they are.

## 5. Document project creation in README.md

```bash
go run golang.org/x/tools/cmd/gonew@latest github.com/telemac/go-service-template github.com/acme/billing
cd billing && make rename setup check
```

Also document:

- how to remove or rename the `customer` and `invoice` examples, file by file
  (`api/<context>/v1`, `internal/<context>`, `newHandler` in
  `cmd/server/main.go`, cases in `cmd/server/main_test.go`);
- add a `LICENSE` if the template is public.

## Done when

`gonew` run against a local copy produces a project where `make rename setup
check` passes without manual edits.
