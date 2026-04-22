BINARY := tsk
PKG    := github.com/Sanjays2402/tsk
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: all build install test lint vet fmt cover clean tidy

all: build

build:
	@mkdir -p dist
	go build -trimpath -ldflags '$(LDFLAGS)' -o dist/$(BINARY) ./cmd/tsk

install:
	go install -trimpath -ldflags '$(LDFLAGS)' ./cmd/tsk

test:
	go test -race -cover ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

vet:
	go vet ./...

fmt:
	gofmt -s -w .

lint:
	golangci-lint run

tidy:
	go mod tidy

clean:
	rm -rf dist coverage.out
