# connect-service-template

Boilerplate for Go services where every entity has one source of truth: a
Protobuf contract served over Connect, gRPC, gRPC-Web and REST (connect-go v2 +
Vanguard), with an OpenAPI document and a typed TypeScript client.

Requirements: Go 1.27+, [buf](https://buf.build/docs/installation), Node.js.

## Start a new project

```bash
go run golang.org/x/tools/cmd/gonew@latest github.com/telemac/connect-service-template github.com/acme/billing
cd billing
make rename setup check
```

`gonew` copies the template under the new module path and rewrites Go
imports; `make rename` points buf's `go_package_prefix`
(`api/buf.gen.yaml`) at the same path. Then:

- set the project name in this README;
- add your first entity by copying `customer` (standard methods) or `invoice`
  (references, lifecycle) — see the recipe in [AGENTS.md](AGENTS.md);
- remove the examples once you no longer need them as reference:
  `api/{customer,invoice}/`, `internal/{customer,invoice}/`, their
  registrations and health names in `newHandler` (`cmd/server/main.go`), the
  tests that use them in `cmd/server/*_test.go`, and `ts/src/client*.ts`
  (rewrite them for your entities).

## Run

```bash
make setup
make check
make run
```

Browser clients on another origin need CORS, off by default:
`go run ./cmd/server -cors-origins=http://localhost:5173`.

gRPC tools work in plaintext (HTTP/2 without TLS); `make run` enables
reflection:

```bash
grpcurl -plaintext localhost:8080 list
grpcurl -plaintext localhost:8080 grpc.health.v1.Health/Check
curl localhost:8080/healthz   # plain HTTP probe
```

```bash
curl -X POST localhost:8080/v1/customers -d '{"name":"Ada","email":"ada@example.com"}'
curl localhost:8080/v1/customers/<id>
curl -X POST localhost:8080/v1/invoices -d '{"customerId":"<id>","amountCents":"1250","currency":"EUR"}'
curl -X POST localhost:8080/v1/invoices/<invoice-id>:issue -d '{}'
curl localhost:8080/openapi.yaml
```

- Why: [docs/intent.md](docs/intent.md)
- How (humans and agents): [AGENTS.md](AGENTS.md), then [docs/patterns.md](docs/patterns.md)
