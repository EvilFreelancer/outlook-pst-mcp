# MCP Specification

## Transport

The server uses MCP over stdio. Messages are JSON-RPC payloads framed with `Content-Length` headers.

The server supports:

- `initialize`
- `notifications/initialized`
- `tools/list`
- `tools/call`

Unknown JSON-RPC methods return a method-not-found error. Tool failures are returned as JSON-RPC errors with actionable messages.

## Tools

### `import_mailbox`

Imports a PST file into the workspace.

Arguments:

- `pst_path`: path to the PST file.

Response:

- `folder_count`
- `message_count`

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

