BINARY  := lazyjail
PKG     := ./cmd/lazyjail
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test test-race lint fmt vet clean

all: build test lint

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

lint: vet
	golangci-lint run ./...
	@test -z "$$(gofmt -l .)" || { echo "gofmt: dosya biçimi hatalı (make fmt)"; gofmt -l .; exit 1; }

clean:
	rm -rf bin dist
