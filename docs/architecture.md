# Architecture Specification

## Overview

The project is a single Go module published as `github.com/EvilFreelancer/outlook-pst-mcp`, with a CLI entrypoint under `cmd/outlook-pst-mcp`. The binary runs an MCP server over stdio. The application service layer coordinates PST import, EML parsing, SQLite metadata, CRUD operations, and final EML export.

## Packages

- `cmd/outlook-pst-mcp`: parses CLI flags, resolves the workspace path, and starts the MCP stdio server.
- `internal/workspace`: resolves the default workspace directory (`.outlook-pst-mcp_data` under the process cwd).
- `internal/mcpserver`: handles MCP JSON-RPC framing, method dispatch, tool listing, and tool calls.
- `internal/app`: exposes mailbox workflows used by MCP tools and integration tests.
- `internal/pst`: validates PST paths, locates `readpst`, runs import extraction, and discovers extracted `.eml` files.
- `internal/store`: owns SQLite schema, repositories, transactions, message pagination, folders, and change history.
- `internal/mail`: parses, builds, and patches EML messages.
- `internal/exporter`: writes the final EML folder tree and `manifest.json`.

## Runtime Flow

1. `main` opens the workspace through `app.Open`.
2. `app.Open` creates the workspace message directory and opens `mailbox.db`.
3. `mcpserver.Server` reads MCP frames from stdin and writes MCP frames to stdout.
4. Tool calls are converted into service calls. `import_pst` runs `readpst` and indexes extracted mail; other tools read or mutate the workspace.
5. Service methods update SQLite and `.eml` files transactionally where practical.
6. Export reads all indexed messages, including deleted messages for counting, and writes only exportable messages by default.

## Data Model

### Folders

Folders have a stable internal ID, optional parent ID, display name, and normalized path. The current implementation stores paths as unique logical paths and uses them during export.

### Messages

Messages have a stable internal ID, owning folder ID, subject, sender, recipients, relative or absolute EML payload path, and a soft-delete flag.

### Changes

Each create, update, move, and delete operation writes a change row with the affected message ID, operation name, JSON payload, and timestamp. The change table is for audit and debugging, not for event replay.

## Dependency Boundaries

The MCP package depends on the application service, not on SQLite or file import internals. The application service composes the lower-level packages. The lower-level packages do not import MCP code.
