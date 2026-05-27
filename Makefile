VERSION ?= dev
GO ?= go
GO_FMT ?= gofmt
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || printf '%s/bin/golangci-lint' "$$($(GO) env GOPATH)")
OUT_DIR ?= .tmp
BINARY = $(OUT_DIR)/modeltap
LDFLAGS = -X main.version=$(VERSION)

.PHONY: all build test lint fmt fmt-check vet clean

all: fmt-check vet test build

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/modeltap/

test:
	$(GO) test -race ./...

lint:
	$(GOLANGCI_LINT) run ./...

fmt:
	$(GO_FMT) -s -w .

fmt-check:
	@files=$$($(GO_FMT) -s -l .); \
	if [ -n "$$files" ]; then \
	  echo "gofmt -s would rewrite the following files:"; \
	  echo "$$files"; \
	  echo "run 'make fmt' to apply."; \
	  exit 1; \
	fi

vet:
	$(GO) vet ./...

clean:
	rm -rf $(OUT_DIR)/modeltap
