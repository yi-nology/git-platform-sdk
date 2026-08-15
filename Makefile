# git-platform-sdk Makefile
#
# Common targets:
#   make test        - run all tests with race detector and coverage
#   make cover       - print coverage summary
#   make lint        - run golangci-lint
#   make fmt         - format all Go files
#   make vet         - run go vet
#   make build       - build all packages
#   make tidy        - run go mod tidy
#   make check       - fmt + vet + lint + test (CI gate)
#   make clean       - remove generated artifacts

GO ?= go
GOLANGCI_LINT ?= golangci-lint
TEST_FLAGS ?= -race -count=1
COVER_PROFILE ?= coverage.out

.PHONY: all build test test-short cover lint fmt vet tidy check clean install-tools

all: build

## build compiles every package in the module.
build:
	$(GO) build ./...

## test runs the full test suite with race detection and coverage.
test:
	$(GO) test $(TEST_FLAGS) -coverprofile=$(COVER_PROFILE) ./...

## test-short runs only tests marked as short (smoke tests).
test-short:
	$(GO) test -short ./...

## cover prints the per-function coverage summary.
cover: test
	$(GO) tool cover -func=$(COVER_PROFILE) | tail -1

## cover-html opens the HTML coverage report in a browser.
cover-html: test
	$(GO) tool cover -html=$(COVER_PROFILE)

## lint runs golangci-lint with the project's configuration.
lint:
	$(GOLANGCI_LINT) run ./...

## fmt formats every Go file with gofmt and goimports.
fmt:
	$(GO) fmt ./...
	@command -v goimports >/dev/null 2>&1 && goimports -w . || true

## vet runs the standard go vet checks.
vet:
	$(GO) vet ./...

## tidy syncs go.mod and go.sum with the current source tree.
tidy:
	$(GO) mod tidy

## check is the CI gate: fmt + vet + lint + test.
check: vet lint test

## clean removes generated artifacts.
clean:
	rm -f $(COVER_PROFILE)

## install-tools installs the development tools listed in tools.go.
install-tools:
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install golang.org/x/tools/cmd/goimports@latest
