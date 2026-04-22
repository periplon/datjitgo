.PHONY: build test lint fmt ci clean install test-update

GO  := go
PKG := ./...
BIN := bin/datjit

build:
	$(GO) build -o $(BIN) ./cmd/datjit

test:
	$(GO) test -race -count=1 $(PKG)

test-update:
	$(GO) test -count=1 -run TestFixtures $(PKG) -update

lint:
	$(GO) vet $(PKG)

fmt:
	$(GO) fmt $(PKG)

ci: fmt lint test build

clean:
	rm -rf bin/ coverage.out

install:
	$(GO) install ./cmd/datjit
