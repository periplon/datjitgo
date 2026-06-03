.PHONY: build check-build test test-fixtures lint fmt check-format ci clean install test-update cover

GO  := go
GOFMT := gofmt
PKG := ./...
# TestFixtures and its -update flag live only in the root package; scoping the
# fixture targets to it avoids spinning up every package's test binary — and
# avoids "flag provided but not defined: -update" from packages that lack the
# flag (which broke `make test-update` under PKG=./...).
FIXTURE_PKG := .
BIN := bin/datjit

build:
	$(GO) build -o $(BIN) ./cmd/datjit

check-build:
	$(GO) build $(PKG)

test:
	$(GO) test -race -count=1 $(PKG)

test-fixtures:
	$(GO) test -count=1 -run TestFixtures $(FIXTURE_PKG)

test-update:
	$(GO) test -count=1 -run TestFixtures $(FIXTURE_PKG) -update

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run $(PKG); \
	else \
		$(GO) vet $(PKG); \
	fi

fmt:
	$(GO) fmt $(PKG)

check-format:
	@test -z "$$($(GOFMT) -l .)" || (echo "gofmt needed:"; $(GOFMT) -l .; exit 1)

cover:
	$(GO) test -race -coverprofile=coverage.out $(PKG) && $(GO) tool cover -func=coverage.out | tail -1

ci: check-format lint test test-fixtures check-build

clean:
	rm -rf bin/ coverage.out

install:
	$(GO) install ./cmd/datjit
