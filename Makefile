GO ?= go
BINARY ?= gandt
VERSION ?= dev

.PHONY: fmt test vet lint run build

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

lint: vet

run:
	$(GO) run ./cmd/gandt

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/$(BINARY) ./cmd/gandt
