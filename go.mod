module github.com/telemac/connectrpc-v2-poc

go 1.27.0

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.12-20260825204119-511051f7f437.2
	connectrpc.com/connect/v2 v2.0.0
	connectrpc.com/cors v0.1.0
	connectrpc.com/validate v0.8.0
	connectrpc.com/vanguard v0.4.1-0.20261007162255-080be557897f
	github.com/rs/cors v1.11.1
	github.com/stretchr/testify v1.12.1
	google.golang.org/genproto/googleapis/api v0.0.0-20260825221802-da73d73af1c5
	google.golang.org/protobuf v1.36.12
)

require (
	buf.build/go/protovalidate v1.4.0 // indirect
	cel.dev/cel-go v0.32.0 // indirect
	cel.dev/expr v0.25.3 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.6.1 // indirect
	github.com/gobwas/glob v1.0.0 // indirect
	github.com/google/gnostic v0.7.1 // indirect
	github.com/google/gnostic-models v0.7.1 // indirect
	github.com/lmittmann/tint v1.2.0 // indirect
	github.com/pb33f/jsonpath v0.8.3 // indirect
	github.com/pb33f/libopenapi v0.38.7 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	github.com/sudorandom/protoc-gen-connect-openapi v0.28.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.6 // indirect
	golang.org/x/exp v0.0.0-20260824195058-e88cd73687aa // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260825221802-da73d73af1c5 // indirect
)

tool (
	connectrpc.com/connect/v2/cmd/protoc-gen-connect-go
	github.com/sudorandom/protoc-gen-connect-openapi
	google.golang.org/protobuf/cmd/protoc-gen-go
)
