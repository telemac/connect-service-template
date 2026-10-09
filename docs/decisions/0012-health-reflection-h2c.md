# 0012 — gRPC health always on, reflection opt-in, HTTP/2 without TLS

- Status: accepted
- Date: 2026-10-09

## Context

Orchestrators and load balancers need a health endpoint, and tools such as
`grpcurl` or AI agents discover an API through gRPC server reflection. The
server only spoke HTTP/1.1, so plaintext gRPC clients could not connect at
all, and reflection needs a bidirectional stream, hence HTTP/2.

## Decision

- `connectrpc.com/grpchealth/v2` is always registered, with a static checker
  for the overall server (`""`) and each API service.
- `connectrpc.com/grpcreflect/v2` is registered only with `-reflection`
  (`make run` enables it): it describes every protobuf type compiled into the
  binary.
- The server enables HTTP/1.1 and unencrypted HTTP/2 (`http.Protocols`,
  h2c). TLS is expected to be terminated in front of the service.

## Consequences

- `grpcurl -plaintext`, `grpc_health_probe` and Kubernetes gRPC probes work
  against the server; plain HTTP clients still get HTTP/1.1.
- Each new service must be added to the health checker in `newHandler`.
- Health is static: it does not reflect dependencies, and is not switched to
  NOT_SERVING during shutdown. Revisit with persistence.
