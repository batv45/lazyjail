BINARY  := lazyjail
PKG     := ./cmd/lazyjail
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test test-race lint fmt vet clean integration

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

# Real fail2ban in Docker: BASE=ubuntu:22.04 | ubuntu:24.04 | debian:12. Needs Docker and root inside the container only.
BASE ?= ubuntu:24.04
integration:
	CGO_ENABLED=0 go test -c -tags integration -o bin/integration.test ./test/integration
	docker build -q --build-arg BASE=$(BASE) -t lazyjail-f2b:$(subst :,-,$(BASE)) test/integration/docker
	docker run --rm -v "$(CURDIR)/bin:/t:ro" lazyjail-f2b:$(subst :,-,$(BASE)) /t/integration.test -test.v -test.timeout 5m

clean:
	rm -rf bin dist
