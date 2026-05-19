BINARY     := asksolr
MODULE     := github.com/codeyogico/asksolr
BUILD_DIR  := dist
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags "-X $(MODULE)/cmd.version=$(VERSION)"

.PHONY: all build test test-unit test-integration lint clean solr-up solr-down solr-wait fmt vet tidy

all: build

## build: compile the CLI binary
build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) .

## run: build and run with args (e.g. make run ARGS="collection list")
run: build
	./$(BUILD_DIR)/$(BINARY) $(ARGS)

## tidy: tidy and vendor go modules
tidy:
	go mod tidy

## fmt: format all Go source files
fmt:
	gofmt -w -s .

## vet: run go vet
vet:
	go vet ./...

## lint: run staticcheck (install: go install honnef.co/go/tools/cmd/staticcheck@latest)
lint:
	staticcheck ./... || true

## test: run unit tests (no build tag required)
test: test-unit

## test-unit: run all unit tests
test-unit:
	go test -v -race -count=1 ./internal/... ./cmd/...

## test-integration: run integration tests against a live Solr instance
## Prerequisites: Solr running at SOLR_URL (default http://localhost:8983/solr)
test-integration:
	go test -v -race -count=1 -tags integration -timeout 120s ./tests/integration/...

## test-all: run unit tests then start Solr and run integration tests
test-all: test-unit solr-up solr-wait test-integration solr-down

## solr-up: start Solr via docker compose
solr-up:
	docker compose up -d
	@echo "Solr starting at http://localhost:8983/solr"

## solr-down: stop and remove Solr containers
solr-down:
	docker compose down -v

## solr-wait: wait until Solr is healthy (up to 60s)
solr-wait:
	@echo "Waiting for Solr to be ready..."
	@for i in $$(seq 1 30); do \
		curl -sf http://localhost:8983/solr/admin/ping > /dev/null 2>&1 && echo "Solr is ready" && exit 0; \
		echo "  attempt $$i/30 — waiting 2s..."; \
		sleep 2; \
	done; \
	echo "ERROR: Solr did not become ready in time" && exit 1

## solr-logs: tail Solr container logs
solr-logs:
	docker compose logs -f solr

## clean: remove build artifacts
clean:
	rm -rf $(BUILD_DIR)

## help: list available targets
help:
	@grep -E '^## [a-z]' Makefile | sed 's/^## //'
