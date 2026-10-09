.PHONY: setup update-deps rename fmt fmt-check generate lint boundaries staticcheck breaking check run clean

STATICCHECK_VERSION := 2026.2.1
# Git ref the contract is compared against; CI passes the pull request base.
BREAKING_AGAINST ?= main
# Generated Go code lives under the module; api/buf.gen.yaml must agree.
GO_PACKAGE_PREFIX := $(shell go list -m)/gen/go

# Installs exactly what the lockfiles pin; buf resolves api/buf.lock on demand.
setup:
	cd ts && npm ci

# Moves buf dependencies to their latest versions; commit api/buf.lock after.
update-deps:
	cd api && buf dep update

# Points buf's go_package prefix at the module in go.mod. gonew rewrites Go
# code only, so run this once after creating a project from the template.
rename:
	sed -i.bak -E 's#(value: ).*/gen/go$$#\1$(GO_PACKAGE_PREFIX)#' api/buf.gen.yaml
	rm api/buf.gen.yaml.bak

# Generated code is not committed: run this after cloning and after every
# change under api/.
generate:
	@grep -q 'value: $(GO_PACKAGE_PREFIX)$$' api/buf.gen.yaml || \
		{ echo "api/buf.gen.yaml go_package_prefix does not match go.mod: run make rename"; exit 1; }
	rm -rf gen ts/gen internal/openapi/openapi.yaml
	cd api && buf generate

# Rewrites Go and proto sources to their canonical format.
fmt:
	gofmt -w cmd internal
	cd api && buf format -w

fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || { gofmt -l cmd internal; echo "Go files not formatted: run make fmt"; exit 1; }
	@cd api && buf format -d --exit-code || { echo "proto files not formatted: run make fmt"; exit 1; }

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

check: fmt-check lint generate boundaries
	go vet ./...
	go test -race ./...
	$(MAKE) staticcheck
	cd ts && npm run typecheck

run: generate
	go run ./cmd/server -reflection

clean:
	rm -rf gen ts/gen ts/dist ts/node_modules internal/openapi/openapi.yaml
