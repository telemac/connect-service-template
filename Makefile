.PHONY: setup generate lint boundaries check run clean

setup:
	cd ts && npm install
	cd api && buf dep update

# Generated code is not committed: run this after cloning and after every
# change under api/.
generate:
	rm -rf gen ts/gen internal/openapi/openapi.yaml
	cd api && buf generate

lint:
	cd api && buf lint

# Domain packages must not import transport or generated contract code; only
# their *rpc adapter packages may.
boundaries:
	@if go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./internal/... \
		| grep -v 'rpc:' \
		| grep -E 'connectrpc\.com|/gen/go/|net/http'; then \
		echo "domain package imports transport or generated code"; exit 1; \
	fi

check: lint generate boundaries
	go vet ./...
	go test -race ./...
	cd ts && npm run typecheck

run: generate
	go run ./cmd/server

clean:
	rm -rf gen ts/gen ts/node_modules internal/openapi/openapi.yaml
