.PHONY: setup update-deps generate lint boundaries staticcheck breaking check run clean

STATICCHECK_VERSION := 2026.2.1
# Git ref the contract is compared against; CI passes the pull request base.
BREAKING_AGAINST ?= main

# Installs exactly what the lockfiles pin; buf resolves api/buf.lock on demand.
setup:
	cd ts && npm ci

# Moves buf dependencies to their latest versions; commit api/buf.lock after.
update-deps:
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

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...

# Fails when api/ changed incompatibly (buf.yaml breaking rules) since
# BREAKING_AGAINST. Not part of check: it needs that ref in the local clone.
breaking:
	buf breaking api --against '.git#ref=$(BREAKING_AGAINST),subdir=api'

check: lint generate boundaries
	go vet ./...
	go test -race ./...
	$(MAKE) staticcheck
	cd ts && npm run typecheck

run: generate
	go run ./cmd/server -reflection

clean:
	rm -rf gen ts/gen ts/node_modules internal/openapi/openapi.yaml
