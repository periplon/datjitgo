.PHONY: build check-build test test-fixtures lint fmt check-format ci clean install test-update

GO  := go
GOFMT := gofmt
PKG := ./...
BIN := bin/datjit

build:
	$(GO) build -o $(BIN) ./cmd/datjit

check-build:
	$(GO) build $(PKG)

test:
	$(GO) test -race -count=1 $(PKG)

test-fixtures:
	$(GO) test -count=1 -run TestFixtures $(PKG)

test-update:
	$(GO) test -count=1 -run TestFixtures $(PKG) -update

lint:
	$(GO) vet $(PKG)

fmt:
	$(GO) fmt $(PKG)

check-format:
	@test -z "$$($(GOFMT) -l .)" || (echo "gofmt needed:"; $(GOFMT) -l .; exit 1)

ci: check-format lint test test-fixtures check-build

clean:
	rm -rf bin/ coverage.out

install:
	$(GO) install ./cmd/datjit
