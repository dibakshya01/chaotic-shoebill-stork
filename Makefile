# chaotic-shoebill-stork — Makefile
# Requires Go >= 1.24 on PATH. (This repo was bootstrapped with Go 1.27.)

SHELL := /bin/bash
BIN   := bin/shoebill
PKG   := github.com/dibakshya01/chaotic-shoebill-stork
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X '$(PKG)/internal/buildinfo.Version=$(VERSION)' \
	-X '$(PKG)/internal/buildinfo.Commit=$(COMMIT)' \
	-X '$(PKG)/internal/buildinfo.Date=$(DATE)'

.DEFAULT_GOAL := help

## help: show this help
.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -e 's/## //' | awk -F ':' '{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

## build: build the shoebill binary into ./bin
.PHONY: build
build:
	@mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/shoebill
	@echo "built $(BIN) ($(VERSION))"

## install: go install the shoebill binary
.PHONY: install
install:
	go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/shoebill

## test: run all unit tests (offline, deterministic)
.PHONY: test
test:
	go test ./... -count=1

## cover: run tests with coverage and print the total
.PHONY: cover
cover:
	go test ./... -count=1 -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1

## race: run tests with the race detector
.PHONY: race
race:
	go test ./... -race -count=1

## demo: run the full detonate->verify->score->sign loop in-process
.PHONY: demo
demo: build
	./$(BIN) demo --seed

## serve: build and serve the dashboard + API with demo data on :8099
.PHONY: serve
serve: build
	./$(BIN) serve --demo --addr :8099

## verify: verify the signatures of the demo evidence ledger
.PHONY: verify
verify: build
	./$(BIN) evidence verify --dir ./.shoebill/evidence

## lint: go vet + staticcheck (staticcheck optional)
.PHONY: lint
lint:
	go vet ./...
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "(staticcheck not installed; skipping)"

## fmt: gofmt the tree
.PHONY: fmt
fmt:
	gofmt -s -w $$(git ls-files '*.go')

## tidy: go mod tidy
.PHONY: tidy
tidy:
	go mod tidy

## catalog-validate: validate every experiment in the catalog
.PHONY: catalog-validate
catalog-validate: build
	./$(BIN) catalog validate ./catalog

## clean: remove build + local state
.PHONY: clean
clean:
	rm -rf bin dist coverage.out coverage.html .shoebill
