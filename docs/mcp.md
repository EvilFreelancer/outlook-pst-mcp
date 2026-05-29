# MCP Specification

## Transport

The server uses MCP over stdio. It accepts both newline-delimited JSON-RPC
messages and JSON-RPC payloads framed with `Content-Length` headers. The
response framing matches the framing used by the first request on the
connection.

Cursor's bundled MCP stdio transport sends newline-delimited JSON. The
`Content-Length` mode is kept for compatibility with MCP clients that use
header-framed stdio messages.

The server supports:

- `initialize`
- `notifications/initialized`
- `tools/list`
- `tools/call`

Unknown JSON-RPC methods return a method-not-found error. Tool failures are returned as JSON-RPC errors with actionable messages.

Tool `inputSchema` values follow MCP 2025-06-18 conventions: plain JSON Schema objects with `type: "object"`, property descriptions, and no `$schema` meta field. Stdout responses are flushed after each framed message so Cursor can complete the initialize handshake promptly.

The MCP serve command opens `mailbox.db` lazily on the first `tools/call` that needs it, not before `initialize`, so the stdio handshake is not blocked by SQLite startup or locks from other processes. The `import_pst` tool may run on an empty workspace.

## Workspace

When `-workspace` is omitted, mailbox state is stored in `<cwd>/.outlook-pst-mcp_data`. Cursor and `make run` should use the repository root as the working directory so each project keeps its own cache. Pass `-workspace <dir>` to override the location.

## Tools

The server starts without mailbox data. Use `import_pst` to load a PST into the workspace, then use the CRUD and search tools. The CLI subcommand `outlook-pst-mcp import` remains available for scripts.

### `import_pst`

Imports messages from a PST file into the workspace using `readpst`. Creates the workspace directory and database on first use.

Arguments:

- `pst_path`: required absolute or relative path to the `.pst` file.

Response:

- `workspace`: workspace directory path.
- `folder_count`: number of folders created or referenced during import.
- `message_count`: number of messages indexed.
- `skipped_count`: number of extracted messages that could not be copied or indexed.

### `list_folders`

Lists indexed folders sorted by path.

Arguments: none.

Response: folder records.

### `list_messages`

Lists message summaries with optional filtering.

Arguments:

- `folder_id`: optional folder filter.
- `query`: optional case-insensitive search text.
- `limit`: optional page size, default 50, maximum 200.
- `offset`: optional page offset, default 0.
- `include_deleted`: optional boolean, default false.

Response:

- `messages`
- `total`

### `get_message`

Reads a message payload.

Arguments:

- `message_id`: required message ID.
- `include_body`: optional boolean.
- `include_headers`: optional boolean.

Response:

- `subject`
- `from`
- `to`
- `cc`
- `body_text`
- `headers`

### `create_message`

Creates a new message in a folder.

Arguments:

- `folder_id`
- `subject`
- `from`
- `to`
- `cc`
- `body_text`
- `headers`

Response:

- `id`

### `update_message`

Patches an existing message.

Arguments:

- `message_id`
- `subject`
- `from`
- `to`
- `cc`
- `body_text`
- `headers`

Response:

- `status`

### `delete_message`

Soft-deletes a message.

Arguments:

- `message_id`

Response:

- `status`

### `move_message`

Moves a message to another folder.

Arguments:

- `message_id`
- `folder_id`

Response:

- `status`

### `export_eml`

Exports the current workspace state as an EML folder tree.

Arguments:

- `output_dir`
- `include_deleted`

Response:

- `status`

## Error Handling

- Missing `readpst` returns installation guidance.
- Invalid PST paths fail before extraction.
- `readpst` failures include the command failure and stderr/stdout tail.
- Unknown tools return a clear unknown-tool error.
- Invalid JSON arguments return JSON-RPC argument errors.
