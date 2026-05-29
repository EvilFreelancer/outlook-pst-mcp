# Go Outlook PST MCP Server Implementation Plan

## Goal

Build a Go MCP server that imports Outlook PST mail through `readpst`, supports CRUD operations in a local workspace, and exports the edited mailbox as an EML folder tree.

## Implemented Structure

- `go.mod`: Go module definition.
- `cmd/outlook-pst-mcp/main.go`: CLI entrypoint and stdio MCP server startup.
- `internal/mail/message.go`: EML parse, build, and patch functions.
- `internal/store/store.go`: SQLite schema, repository methods, and transactions.
- `internal/pst/importer.go`: `readpst` lookup, command execution, extracted EML discovery.
- `internal/exporter/exporter.go`: EML tree export and manifest writing.
- `internal/app/service.go`: mailbox application workflows used by MCP tools and tests.
- `internal/mcpserver/server.go`: JSON-RPC MCP protocol handling and tool dispatch.
- `README.md`: English documentation for purpose, usage, running, installing, and building.

## Completed Tasks

- Project skeleton with Go module and package structure.
- EML handling tests and implementation.
- SQLite store tests and implementation.
- PST importer tests with fake `readpst` and implementation.
- Application service workflow tests and implementation.
- Exporter tests and implementation.
- MCP server tests for tool listing, tool calls, and stdio framing.
- README and Makefile documentation.

## Verification Commands

```bash
make check
```

This runs Go static analysis. For full verification, also run:

```bash
make test
make build
```
