# Outlook PST MCP Server

This project is a local MCP server for working with messages from an Outlook PST archive.

The server treats the source PST file as read-only input. It imports messages through `readpst`, stores editable mailbox state in a workspace, exposes CRUD tools over MCP, and exports the final mailbox state as an EML folder tree with a `manifest.json` file.

Repository: `https://github.com/EvilFreelancer/outlook-pst-mcp`

## What It Does

- Imports Outlook PST content through `readpst` from `libpst`.
- Indexes folders and messages in SQLite.
- Stores editable message payloads as canonical `.eml` files.
- Exposes MCP tools for folder browsing, message search, reading, creating, updating, deleting, moving, and exporting messages.
- Exports edited state as an EML directory tree that can be imported by mail software.

## What It Does Not Do

- It does not modify the original PST file.
- It does not write a new PST file.
- It does not connect to Outlook, Exchange, IMAP, Microsoft Graph, or any remote mailbox.
- It does not provide a web UI.

## Requirements

- Go 1.24 or newer.
- GCC and CGO support for the SQLite driver.
- `readpst` installed and available on `PATH` for real PST imports.

On Debian or Ubuntu, `readpst` is usually provided by `pst-utils`:

```bash
sudo apt install pst-utils
```

## Build

Clone the repository and enter the project directory:

```bash
git clone https://github.com/EvilFreelancer/outlook-pst-mcp.git
cd outlook-pst-mcp
```

```bash
make build
```

The command creates `bin/outlook-pst-mcp`.

## Install

Install to `~/.local/bin`:

```bash
make install
```

Without parameters, `make install` installs:

```text
~/.local/bin/outlook-pst-mcp
```

Install to another prefix:

```bash
make install PREFIX=/usr/local
```

Install to an exact binary directory:

```bash
make install BINDIR=/custom/bin
```

## Run

Run the MCP server over stdio:

```bash
make run
```

Without `-workspace`, data is stored under `.outlook-pst-mcp_data` in the process current working directory (the project root when Cursor starts the MCP server from this repo).

The workspace stores:

```text
.outlook-pst-mcp_data/
  mailbox.db
  extracted/
  messages/
  export/
```

## Import a PST

Use the `import_pst` MCP tool from a connected client, or the CLI subcommand:

```bash
./bin/outlook-pst-mcp import -pst /absolute/path/to/backup.pst
```

Both write into the same workspace database (default `.outlook-pst-mcp_data`).

## MCP Client Configuration

Example client configuration:

```json
{
  "mcpServers": {
    "outlook-pst": {
      "command": "/absolute/path/to/outlook-pst-mcp"
    }
  }
}
```

Add `"args": ["-workspace", "/path/to/dir"]` only when the mailbox state should not use the project default `.outlook-pst-mcp_data`.

Cursor uses newline-delimited JSON for stdio MCP servers. The server also
accepts `Content-Length` framed messages for compatibility with other clients
that use header-framed stdio. After changing server code, run `make build` before
reloading the MCP server in Cursor so the configured binary is up to date.

## Tools

### `import_pst`

Imports a PST file into the workspace. Use this when the server has no mailbox data yet.

```json
{
  "pst_path": "/absolute/path/to/backup.pst"
}
```

Response fields: `workspace`, `folder_count`, `message_count`, `skipped_count`.

### `list_folders`

Lists indexed mailbox folders.

```json
{}
```

### `list_messages`

Lists message summaries.

```json
{
  "folder_id": "fld_...",
  "query": "invoice",
  "limit": 50,
  "offset": 0,
  "include_deleted": false
}
```

### `get_message`

Reads one message.

```json
{
  "message_id": "msg_...",
  "include_body": true,
  "include_headers": false
}
```

### `create_message`

Creates a new message in a folder.

```json
{
  "folder_id": "fld_...",
  "subject": "New message",
  "from": "sender@example.com",
  "to": ["recipient@example.com"],
  "cc": [],
  "body_text": "Message body",
  "headers": {
    "X-Source": "outlook-pst-mcp"
  }
}
```

### `update_message`

Updates message fields.

```json
{
  "message_id": "msg_...",
  "subject": "Updated subject",
  "body_text": "Updated body"
}
```

### `delete_message`

Soft-deletes a message. The EML payload remains on disk, but the message is skipped during export by default.

```json
{
  "message_id": "msg_..."
}
```

### `move_message`

Moves a message to another folder.

```json
{
  "message_id": "msg_...",
  "folder_id": "fld_..."
}
```

### `export_eml`

Exports the current workspace state as an EML folder tree.

```json
{
  "output_dir": "/absolute/path/to/export",
  "include_deleted": false
}
```

## Test

```bash
make test
```

`make test` uses `GOCACHE=/tmp/outlook-pst-mcp-go-build` by default so tests work in restricted environments.

## Make Targets

```bash
make help
make fmt
make check
make test
make build
make install
make run
make clean
```

`make check` runs `go vet ./...`.

## Documentation

Project documentation lives in `docs/`:

- `docs/product.md`: product goal, constraints, non-goals, and workflow.
- `docs/architecture.md`: packages, runtime flow, data model, and boundaries.
- `docs/file-workflow.md`: PST, `readpst`, EML, SQLite, and export file handling.
- `docs/mcp.md`: MCP transport, tools, arguments, responses, and errors.
- `docs/development.md`: workflow, Makefile targets, test, build, and install behavior.
- `docs/implementation-plan.md`: implementation plan and completed task map.

## Development Notes

- Project-facing text is written in English.
- Documentation lives in `docs/`.
- Tests are written before implementation code.
- The source PST file is always treated as read-only input.
- Use `make check`, `make test`, and `make build` before considering changes complete.
