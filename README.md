# connectrpc-v2-poc

Boilerplate for Go services where every entity has one source of truth: a
Protobuf contract served over Connect, gRPC, gRPC-Web and REST (connect-go v2 +
Vanguard), with an OpenAPI document and a typed TypeScript client.

Requirements: Go 1.27+, [buf](https://buf.build/docs/installation), Node.js.

```bash
make setup
make check
make run
```

Browser clients on another origin need CORS, off by default:
`go run ./cmd/server -cors-origins=http://localhost:5173`.

```bash
curl -X POST localhost:8080/v1/customers -d '{"name":"Ada","email":"ada@example.com"}'
curl localhost:8080/v1/customers/<id>
curl -X POST localhost:8080/v1/invoices -d '{"customerId":"<id>","amountCents":"1250","currency":"EUR"}'
curl -X POST localhost:8080/v1/invoices/<invoice-id>:issue -d '{}'
curl localhost:8080/openapi.yaml
```

- Why: [docs/intent.md](docs/intent.md)
- How (humans and agents): [AGENTS.md](AGENTS.md), then [docs/patterns.md](docs/patterns.md)
