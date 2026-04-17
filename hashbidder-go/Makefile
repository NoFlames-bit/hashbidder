# Makefile for hashbidder-go (run from this directory).
#
# Targets:
#   build / all     — compile CLI to bin/hashbidder
#   test            — go test ./...
#   test-race       — tests with -race (needs CGO on many platforms)
#   coverage        — coverprofile + per-func summary (no race)
#   coverage-html   — coverage.out + coverage.html (opens in browser)
#   test-race-coverage — -race and -coverprofile in one go test run
#   lint            — run golangci-lint on all packages
#   check           — fmt, vet, lint, then test-race-coverage
#   clean           — remove coverage artifacts and bin/
.PHONY: all build fmt vet lint test test-race coverage coverage-html test-race-coverage check clean

GO      ?= go
PKG     ?= ./...
COVER   ?= coverage.out
COVERMODE ?= atomic

all: build

build:
	$(GO) build -o bin/hashbidder ./cmd/hashbidder

fmt:
	$(GO) fmt $(PKG)

vet:
	$(GO) vet $(PKG)

lint:
	golangci-lint run ./...

test:
	$(GO) test $(PKG)

# Race detector: slower; catches data races. Requires cgo on many platforms.
test-race:
	CGO_ENABLED=1 $(GO) test -race $(PKG)

# Coverage report (terminal). Uses atomic covermode so counts are safe if tests use -parallel.
coverage:
	$(GO) test -coverprofile=$(COVER) -covermode=$(COVERMODE) $(PKG)
	$(GO) tool cover -func=$(COVER)

coverage-html: coverage
	$(GO) tool cover -html=$(COVER) -o coverage.html
	@echo "Wrote coverage.html"

# Race + coverage in one run (faster than separate test-race + coverage).
test-race-coverage:
	CGO_ENABLED=1 $(GO) test -race -coverprofile=$(COVER) -covermode=$(COVERMODE) $(PKG)
	$(GO) tool cover -func=$(COVER)

check: fmt vet lint test-race-coverage

clean:
	rm -f $(COVER) coverage.html
	rm -rf bin
