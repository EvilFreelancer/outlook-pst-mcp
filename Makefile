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
