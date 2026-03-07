VERSION ?= dev
GO ?= /usr/local/opt/go/bin/go
BINARY = bin/modeltap
LDFLAGS = -X main.version=$(VERSION)

.PHONY: all build test lint fmt vet clean

all: fmt vet lint test build

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/modeltap/

test:
	$(GO) test -race ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .

vet:
	$(GO) vet ./...

clean:
	rm -rf bin/
