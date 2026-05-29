GO ?= go
GOCACHE ?= /tmp/email-parsing-go-build
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
WORKSPACE ?= ./workspace
BIN_DIR := bin
BIN := $(BIN_DIR)/outlook-pst-mcp
CMD := ./cmd/outlook-pst-mcp
GOFLAGS := -buildvcs=false

.PHONY: help fmt check test build install run clean

help:
	@printf '%s\n' \
	  'Targets:' \
	  '  make fmt                         format Go sources' \
	  '  make check                       run go vet' \
	  '  make test                        run all tests' \
	  '  make build                       build bin/outlook-pst-mcp' \
	  '  make install [PREFIX|BINDIR=...] install the binary' \
	  '  make run WORKSPACE=./workspace   run the MCP server over stdio' \
	  '  make clean                       remove build output'

build:
	mkdir -p $(BIN_DIR)
	GOCACHE=$(GOCACHE) $(GO) build $(GOFLAGS) -o $(BIN) $(CMD)

fmt:
	gofmt -w cmd internal

vet:
	GOCACHE=$(GOCACHE) $(GO) vet ./...

check: vet

test:
	GOCACHE=$(GOCACHE) $(GO) test ./...

install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 0755 $(BIN) $(DESTDIR)$(BINDIR)/outlook-pst-mcp

run:
	GOCACHE=$(GOCACHE) $(GO) run $(GOFLAGS) $(CMD) -workspace $(WORKSPACE)

clean:
	rm -rf $(BIN_DIR)
